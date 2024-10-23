package properties

import (
	"fmt"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"sync"
	"time"

	"toolkit/dispatch"
)

const UpdateTopic = "observablePropertyUpdated"

type ObservableProperty[T interface{}] struct {
	mux        *sync.Mutex
	name       *string
	value      *T
	dispatcher *dispatch.Dispatcher
	updateTime *time.Time
}

func (o *ObservableProperty[T]) Set(value *T) {
	o.value = value
	o.updateTime = util.PTime(time.Now())
	if o.dispatcher != nil {
		o.dispatcher.Dispatch(UpdateTopic+"."+*o.name, o)
	}
}

func (o *ObservableProperty[T]) Get() *T {
	return o.value
}

func (o *ObservableProperty[T]) GetCritical() *T {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.value
}

func (o *ObservableProperty[T]) Value() T {
	if o.value == nil {
		panic("Value is unnassigned")
	} else {
		return *o.value
	}
}

func (o *ObservableProperty[T]) Copy() *T {
	if o.value == nil {
		return nil
	} else {
		v := *o.value
		valueOf := reflect.ValueOf(v)
		if valueOf.Kind() == reflect.Struct {
			valueOf = util.CopyStructValue(valueOf)
			return valueOf.Addr().Interface().(*T)
		} else if valueOf.Kind() == reflect.Map {
			tmp := util.CopyMapValue(valueOf).Interface().(T)
			return &tmp
		} else if valueOf.Kind() == reflect.Slice {
			tmp := util.CopySliceValue(valueOf).Interface().(T)
			return &tmp
		} else {
			return &v
		}
	}
}

/*
*
Deep copies structs, maps and arrays only. It doed NOT copy simple type pointers.
*/
func (o *ObservableProperty[T]) Snapshot() *T {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.Copy()
}

func (o *ObservableProperty[T]) SnapshotWithTime() (*T, *time.Time) {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.Copy(), o.updateTime
}

func (o *ObservableProperty[T]) Lock() *T {
	o.mux.Lock()
	return o.Get()
}

func (o *ObservableProperty[T]) LockWithCopy() *T {
	o.mux.Lock()
	return o.Copy()
}

func (o *ObservableProperty[T]) UnLock() {
	o.mux.Unlock()
}

func (o *ObservableProperty[T]) RegisterListener(l dispatch.Listener) {
	o.createDispatcher()
	o.dispatcher.RegisterListener(l, UpdateTopic)
}

func (o *ObservableProperty[T]) createDispatcher() {
	o.mux.Lock()
	defer o.mux.Unlock()
	if o.dispatcher == nil {
		o.dispatcher = dispatch.NewDispatcher()
	}
}

func (o *ObservableProperty[T]) String() string {
	if o.value == nil {
		return fmt.Sprintf("%s: nil", *o.name)
	} else {
		return fmt.Sprintf("%s: %v", *o.name, o.value)
	}
}

func (o *ObservableProperty[T]) Assigned() bool {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.value != nil
}

func (o *ObservableProperty[T]) SetVal(v T) {
	o.Set(&v)
}

func (o *ObservableProperty[T]) SetCritical(v *T) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.Set(v)
}

func NewObservableProperty[T interface{}](name string, dispatcher *dispatch.Dispatcher) *ObservableProperty[T] {
	return &ObservableProperty[T]{name: &name, value: nil, mux: &sync.Mutex{}, dispatcher: dispatcher}
}
