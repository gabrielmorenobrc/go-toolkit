package remoting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"toolkit/web"
)

type ExecutionRequestData struct {
	FunctionName *string
	Arguments    json.RawMessage `required:"false"`
}

type ExecutionResponseData struct {
	Result json.RawMessage `required:"false"`
}

type AgentEndpoints struct {
	descriptors *Descriptors
}

func (o *AgentEndpoints) execute(w http.ResponseWriter, r *http.Request) {
	data := ExecutionRequestData{}
	web.ParseParamOrBody(r, &data)
	web.ValidateStruct(data)
	h := o.descriptors.Find(*data.FunctionName)
	if h == nil {
		panic(fmt.Sprintf("No handler found for function name %s", *data.FunctionName))
	}
	argumentInstances := o.resolveArgumentInstances(*h)
	util.Unmarshal(data.Arguments, &argumentInstances)
	result := o.invokeHandler(argumentInstances, *h)
	responseData := ExecutionResponseData{}
	if result != nil {
		responseData.Result = util.Marshal(result)
	}
	web.JsonResponse(responseData, w)
}

func (o *AgentEndpoints) invokeHandler(argumentInstances []interface{}, h HandlerDescriptor) interface{} {
	arguments := make([]reflect.Value, len(argumentInstances))
	for i, a := range argumentInstances {
		arguments[i] = reflect.ValueOf(a).Elem()
	}
	out := reflect.ValueOf(h.Handler).Call(arguments)
	if len(out) > 0 {
		return out[0].Interface()
	}
	return nil
}

func (o *AgentEndpoints) resolveArgumentInstances(descriptor HandlerDescriptor) []interface{} {
	t := reflect.TypeOf(descriptor.Handler)
	result := make([]interface{}, t.NumIn())
	for i := 0; i < len(result); i++ {
		ti := t.In(i)
		result[i] = reflect.New(ti).Interface()
	}
	return result
}

func (o *AgentEndpoints) ConfigureHandlers(mux *http.ServeMux, prefix string) {
	web.HandleDefault(mux, prefix+"/execute", o.execute)
}

func NewAgentEndpoints(descriptors *Descriptors) *AgentEndpoints {
	return &AgentEndpoints{descriptors: descriptors}
}
