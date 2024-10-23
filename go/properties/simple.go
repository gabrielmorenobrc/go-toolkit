package properties

import (
	"reflect"
	"sparrowhawktech/toolkit/util"
	"sync"
	"time"
)

type SimpleProperty[T interface{}] struct {
	mux         sync.Mutex
	value       *T
	updatedTime *time.Time
}

func (o *SimpleProperty[T]) Set(value *T) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.value = value
	o.updatedTime = util.PTime(time.Now())
}

func (o *SimpleProperty[T]) SetVal(value T) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.value = &value
	o.updatedTime = util.PTime(time.Now())
}

func (o *SimpleProperty[T]) Get() *T {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.value
}

func (o *SimpleProperty[T]) SnapshotWithTime() (*T, *time.Time) {
	o.mux.Lock()
	defer o.mux.Unlock()
	value := o.copy()
	var t *time.Time
	if o.updatedTime != nil {
		v := *o.updatedTime
		t = &v
	}
	return value, t
}

/*
*
Deep copies structs, maps and arrays only. It doed NOT copy simple type pointers.
*/
func (o *SimpleProperty[T]) Snapshot() *T {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.copy()
}

func (o *SimpleProperty[T]) copy() *T {
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

func (o *SimpleProperty[T]) UpdatedTime() *time.Time {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.updatedTime
}
