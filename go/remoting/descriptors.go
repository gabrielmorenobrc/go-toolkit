package remoting

import (
	"fmt"
	"reflect"
	"sync"
)

type HandlerDescriptor struct {
	Type    reflect.Type
	Handler interface{}
}

type Descriptors struct {
	mux           *sync.Mutex
	descriptorMap map[string]HandlerDescriptor
}

func (o *Descriptors) Register(t reflect.Type, handler interface{}) {
	o.mux.Lock()
	defer o.mux.Unlock()
	name := t.Name()
	if _, ok := o.descriptorMap[name]; ok {
		panic(fmt.Sprintf("handler already registered for name %s", name))
	}
	o.descriptorMap[name] = HandlerDescriptor{
		Type:    t,
		Handler: handler,
	}
}

func (o *Descriptors) Find(name string) *HandlerDescriptor {
	o.mux.Lock()
	defer o.mux.Unlock()
	if h, ok := o.descriptorMap[name]; ok {
		return &h
	} else {
		return nil
	}
}

func NewDescriptors() *Descriptors {
	return &Descriptors{
		mux:           &sync.Mutex{},
		descriptorMap: make(map[string]HandlerDescriptor),
	}
}
