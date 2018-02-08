package Catch

import (
	"github.com/gin-gonic/gin"
	"errors"
	"net/url"
	"net/http"
	"bytes"
	"encoding/json"
	"strconv"
	"fmt"
)

const JsonByteStreamHeader = "application/json; charset=utf-8"
const FormEncodedHeader = "application/x-www-form-urlencoded; charset=utf-8"

//gets the byte stream however parses it as an error if the status is failing
func GetRespByteStream(response *http.Response) ([]byte, int, error){
	buf := bytes.NewBuffer(make([]byte, 0, response.ContentLength))
	_, _ = buf.ReadFrom(response.Body)

	if response.StatusCode != 200 {
		values,_:=url.ParseQuery(buf.String())
		response.Body.Close()
		return buf.Bytes(),response.StatusCode,errors.New(values.Get("error"))
	} else {
		return buf.Bytes(),200,nil
	}
}

//takes an error and serialises it to the body as a form value
func HandleRequestErrors(c *gin.Context, code int,err error) {
	if err != nil {
		v := url.Values{}
		v.Add("error",err.Error())
		c.String(code,v.Encode())
	} else {
		c.String(code,"Encountered unknown error")
	}
}

type fullResponse struct {
	Body interface{}
	Log Log
}

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

func CreateRectifierWithPath(method string, domain string, path string, query string, req interface{}) (Rectifier){
	var rectifier Rectifier
	rectifier.Method = method
	rectifier.Rectify = req
	rectifier.TargetDomain = fmt.Sprintf("%s%s",domain,path)
	rectifier.TargetQuery = query
	return rectifier
}

func (g *Rectifier) EnactRectifier() (interface{}, int, Log, error){
	toBeSentBytes, _ := json.Marshal(g)

	req, formErr := http.NewRequest(g.Method,fmt.Sprintf("%s?%s",g.TargetDomain,g.TargetQuery), bytes.NewBuffer(toBeSentBytes))
	if formErr != nil {
		return nil, 500, Log{}, formErr
	}

	resp, err := http.DefaultClient.Do(req)
	if err !=  nil {
		return nil, 500, Log{},err
	}

	object, code, extLog := ReceiveResponse(resp)

	return object, code, extLog,nil
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

func CreateFailureFromError(code int, origin string,originError error,isFatal bool,rectifier Rectifier) (Failure){
	var newFail Failure
	newFail.Code = code
	newFail.Origin = origin
	newFail.Message = originError.Error()
	newFail.Fatal = isFatal
	newFail.Rectifier = rectifier

	return newFail
}

//Function to handle success and return them to sender
func SendResponse(c *gin.Context, code int, log Log, bodyContent interface{}){
	toBeSent := new(fullResponse)
	toBeSent.Log = log
	toBeSent.Body = bodyContent
	bodyBytes, _ := json.Marshal(toBeSent)
	c.Header("Content-Length", strconv.Itoa(len(bodyBytes)))

	c.Data(code,JsonByteStreamHeader,bodyBytes)
}

//Function to handle errors and send them to logging service
func ReceiveResponse(response *http.Response)(interface{}, int, Log){
	buf := bytes.NewBuffer(make([]byte, 0, response.ContentLength))
	_, _ = buf.ReadFrom(response.Body)

	toBeReceived := new(fullResponse)

	json.Unmarshal(buf.Bytes(),&toBeReceived)
	fmt.Println("Catch")
	fmt.Println(toBeReceived)
	fmt.Println(buf.Bytes())
	fmt.Println("End")
	return toBeReceived.Body, response.StatusCode, toBeReceived.Log
}

//Function to parse byte stream and decode Log
func HandleKnockout(c *gin.Context,code int, log Log){
	//todo add service sending of error here
	SendResponse(c,code,log,nil)
}

func HandleKnockoutPunch(c *gin.Context,code int, origin string, punch error){
	var log Log
	var nilRectifier Rectifier
	log.AddNewFailureFromError(code,origin,punch,true,nilRectifier)
	HandleKnockout(c,code,log)
}
