package remoting_test

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"testing"
	"time"
	"toolkit/remoting"
)

type Fede struct {
	Not *string
}

type EvaluateFede func(a string, b int, c Fede) *Fede

type DomainSpecificApi struct {
	EvaluateFede EvaluateFede
}

type DomainSpecificAgent struct {
	agentEndpoints *remoting.AgentEndpoints
	Descriptors    *remoting.Descriptors
	port           *int
}

func (o *DomainSpecificAgent) Start() {
	serveMux := http.NewServeMux()
	o.agentEndpoints.ConfigureHandlers(serveMux, "/agent")
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", *o.port), Handler: serveMux}
	err := httpServer.ListenAndServe()
	util.CheckErr(err)
}

func (o *DomainSpecificAgent) registerHandlers() {
	api := DomainSpecificApi{}
	o.Descriptors.Register(reflect.TypeOf(api.EvaluateFede), o.evaluateFede)
}

func (o *DomainSpecificAgent) evaluateFede(a string, b int, c Fede) *Fede {
	println(a, b, string(util.Marshal(c)))
	return &Fede{
		Not: util.PStr("too smart"),
	}
}

func NewDomainSpecificAgent(port int) *DomainSpecificAgent {
	handlers := remoting.NewDescriptors()
	a := DomainSpecificAgent{Descriptors: handlers, agentEndpoints: remoting.NewAgentEndpoints(handlers), port: &port}
	a.registerHandlers()
	return &a
}

type DomainSpecificGateway struct {
	gateway *remoting.Gateway
}

func (o *DomainSpecificGateway) initAPI(api *DomainSpecificApi, host string, timeout time.Duration) {
	api.EvaluateFede = o.makeFunc(reflect.TypeOf(api.EvaluateFede), host, timeout).(EvaluateFede)
}

func (o *DomainSpecificGateway) makeFunc(of reflect.Type, host string, timeout time.Duration) interface{} {
	v := reflect.MakeFunc(of, func(args []reflect.Value) (results []reflect.Value) {
		arguments := make([]interface{}, len(args))
		for i := 0; i < len(args); i++ {
			arguments[i] = args[i].Interface()
		}
		return []reflect.Value{reflect.ValueOf(o.gateway.Execute(host, of.Name(), arguments, timeout))}
	})
	return v.Interface()
}

func (o *DomainSpecificGateway) buildDescriptors(api DomainSpecificApi) map[string]reflect.Type {
	descriptors := make(map[string]reflect.Type)
	t := reflect.TypeOf(api)
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		descriptors[ft.Name] = ft.Type
	}
	return descriptors
}

func (o *DomainSpecificGateway) Api(host string, timeout time.Duration) DomainSpecificApi {
	api := DomainSpecificApi{}
	o.initAPI(&api, host, timeout)
	return api
}

func NewDomainSpecificGateway(port int) *DomainSpecificGateway {
	o := &DomainSpecificGateway{}
	descriptors := o.buildDescriptors(DomainSpecificApi{})
	o.gateway = remoting.NewGateway(port, "/agent", descriptors)
	return o
}

func TestAll(t *testing.T) {
	port := 9090
	agent := NewDomainSpecificAgent(port)
	gateway := NewDomainSpecificGateway(port)
	go agent.Start()

	time.Sleep(time.Second)
	fede := gateway.Api("localhost", time.Minute).EvaluateFede("a", 1, Fede{Not: util.PStr("too pretty")})
	util.JsonPretty(fede, os.Stdout)

}
