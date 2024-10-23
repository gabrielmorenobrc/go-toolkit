package serialization

import (
	"reflect"
)

const (
	Assigned    = 1
	NotAssigned = 0
)

var byteArrayType = reflect.TypeOf([]byte{})
