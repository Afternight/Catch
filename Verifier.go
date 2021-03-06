package Catch

import (
	"github.com/gin-gonic/gin"
	"encoding/json"
	"strconv"
	"fmt"
)

const JsonByteStreamHeader = "application/json; charset=utf-8"
const FormEncodedHeader = "application/x-www-form-urlencoded; charset=utf-8"

//Log type

//Log is the high level collection of whats happened
//Intended to be passed to functions to add failures as nessasary
//Does not contain debug information for the log etc, as this is handled on fatality
//Failures on addition can knockout the log causing a fatality
//FatalityHandler is what handles when this happens, terminating the request and sending log to the service
type Log struct {
	Fatality bool
	Failures []Failure
	Messages []string
}

func (g *Log) MergeLogs(newLog Log) {
	if newLog.Fatality != false {
		g.Fatality = true
	}

	g.Failures = append(g.Failures,newLog.Failures...)
	g.Messages = append(g.Messages,newLog.Messages...)
}

func (g *Log) AddFailure(fail Failure){
	if fail.Fatal == true {
		g.Fatality = true
	}
	g.Failures =  append(g.Failures, fail)
}

func (g *Log) AddNewFailureFromError(code int, origin string, originError error,isFatal bool, rectifier Rectifier){
	newFailure := CreateFailureFromError(code,origin,originError,isFatal,rectifier)
	g.AddFailure(newFailure)
}

func (g *Log) GetLog() (Log){
	return *g
}

type Failure struct {
	Code int
	Origin string
	Message string
	Fatal bool
	Rectifier Rectifier
}

type Rectifier struct {
	Rectify interface{} //this is left interfacial for the purpose of usage elsewhere
	TargetDomain string
	TargetQuery string
	Method string
}

type IsLogged interface {
	GetLog()(Log)
}

func CreateRectifierWithPath(method string, domain string, path string, query string, req interface{}) (Rectifier){
	var rectifier Rectifier
	rectifier.Method = method
	rectifier.Rectify = req
	rectifier.TargetDomain = fmt.Sprintf("%s%s",domain,path)
	rectifier.TargetQuery = query
	return rectifier
}

func CreateFailureFromError(code int, origin string,originError error,isFatal bool,rectifier Rectifier) (Failure){
	var newFail Failure
	newFail.Code = code
	newFail.Origin = origin
	newFail.Message = originError.Error()
	newFail.Fatal = isFatal
	newFail.Rectifier = rectifier

	return newFail
}

//Function to knockout due to an error, sending back the isLogged object (can just be log or full object in itself)
func HandleKnockout(c *gin.Context,code int, obj IsLogged){
	//todo add service sending of error here
	sendBytes, _ :=json.Marshal(obj)
	c.Header("Content-Length", strconv.Itoa(len(sendBytes)))
	c.Data(code,JsonByteStreamHeader,sendBytes)
}

//A knockout punch is a non-rectifiable error persistent to the crt sent
//ie a formatting issue or such that the we cannot create a rectifier
func HandleKnockoutPunch(c *gin.Context,code int, origin string, punch error){
	log := new(Log)
	var nilRectifier Rectifier
	log.AddNewFailureFromError(code,origin,punch,true,nilRectifier)
	HandleKnockout(c,code,log)
}


