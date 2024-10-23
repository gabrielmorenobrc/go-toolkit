package remoting

import (
	"errors"
	"fmt"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"time"
	"toolkit/web"
)

var errorType = reflect.TypeOf(errors.New(""))

type ExecutionRequest struct {
	HostAddress  *string
	FunctionName *string
	Input        interface{}
	Timeout      time.Duration
}

type Gateway struct {
	descriptors map[string]reflect.Type
	agentsPort  *int
	contextPath *string
}

func (o *Gateway) Execute(host string, functionName string, arguments []interface{}, timeout time.Duration) interface{} {
	requestData := ExecutionRequestData{}
	requestData.FunctionName = &functionName
	jsonBytes := util.Marshal(arguments)
	requestData.Arguments = jsonBytes
	handlerType, ok := o.descriptors[functionName]
	if !ok {
		panic(fmt.Sprintf("Unregistered function name %s", functionName))
	}
	responseData := ExecutionResponseData{}

	url := fmt.Sprintf("http://%s:%d%s/execute", host, *o.agentsPort, *o.contextPath)
	web.PostJson(url, requestData, &responseData, 200, timeout, nil)
	if responseData.Result != nil {
		result := o.resolveResultInstance(handlerType)
		util.Unmarshal(responseData.Result, result)
		return reflect.ValueOf(result).Elem().Interface()
	} else {
		return nil
	}

}

func (o *Gateway) resolveResultInstance(t reflect.Type) interface{} {
	if t.NumOut() == 0 {
		return nil
	} else if t.NumOut() > 1 {
		panic("nooooope, we want just one result")
	}
	to := t.Out(0)
	if to.AssignableTo(errorType) {
		panic("nooooope, we don't want error results. we want panics.")
	}
	return reflect.New(to).Interface()

}

func NewGateway(agentsPort int, contextPath string, descriptors map[string]reflect.Type) *Gateway {
	return &Gateway{agentsPort: &agentsPort, contextPath: &contextPath, descriptors: descriptors}
}
