package serialization

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"time"
)

type Encoder struct {
	buf64      []byte
	assigned   []byte
	unassigned []byte
}

func (o *Encoder) StoreUint64(w io.Writer, value *uint64) {
	if o.StoreAssigned(w, value) {
		o.WriteUint64Value(w, *value)
	}
}

func (o *Encoder) StoreUint32Slice(w io.Writer, value []uint32) {
	if o.StoreAssigned(w, value) {
		o.WriteUint64Value(w, uint64(len(value)))
		for _, u32 := range value {
			o.WriteUint64Value(w, uint64(u32))
		}
	}
}

func (o *Encoder) StoreAssigned(w io.Writer, v interface{}) bool {
	isNil := v == nil
	if !isNil {
		rv := reflect.ValueOf(v)
		canIsNil := util.CanIsNil(rv)
		isNil = (canIsNil && rv.IsNil()) || (!canIsNil && rv.IsZero())
	}

	if isNil {
		o.write(w, o.unassigned)
		return false
	} else {
		o.write(w, o.assigned)
		return true
	}
}

func (o *Encoder) write(w io.Writer, data []byte) {
	l, err := w.Write(data)
	util.CheckErr(err)
	if l != len(data) {
		panic(fmt.Sprintf("written length does not match: %d vs %d", l, len(data)))
	}
}

func (o *Encoder) StoreInt64(w io.Writer, value *int64) {
	if o.StoreAssigned(w, value) {
		o.WriteInt64Value(w, *value)
	}
}

func (o *Encoder) WriteUint64Value(w io.Writer, value uint64) {
	binary.BigEndian.PutUint64(o.buf64, value)
	o.write(w, o.buf64)
}

func (o *Encoder) WriteInt64Value(w io.Writer, value int64) {
	o.WriteUint64Value(w, uint64(value))
}

func (o *Encoder) WriteIntValue(w io.Writer, value int) {
	o.WriteUint64Value(w, uint64(value))
}

func (o *Encoder) WriteUintValue(w io.Writer, value uint) {
	o.WriteUint64Value(w, uint64(value))
}

func (o *Encoder) WriteFloat64Value(w io.Writer, value float64) {
	bits := math.Float64bits(value)
	o.WriteUint64Value(w, bits)
}

func (o *Encoder) StoreStruct(w io.Writer, s interface{}) {
	value := reflect.ValueOf(s)
	o.storeField(w, value)
}

func (o *Encoder) writeStructValue(w io.Writer, value reflect.Value) {
	objectType := value.Type()
	for i := 0; i < objectType.NumField(); i++ {
		field := objectType.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := value.Field(i)
		o.storeField(w, fv)
	}
}

func (o *Encoder) storeField(w io.Writer, value reflect.Value) {
	if !o.StoreAssigned(w, value.Interface()) {
		return
	}
	objectType := value.Type()
	isPtr := objectType.Kind() == reflect.Ptr
	if isPtr {
		value = value.Elem()
	}
	if util.IsStructValue(value) {
		o.writeStructValue(w, value)
	} else if objectType.Kind() == reflect.Map {
		o.writeMapValue(w, value)
	} else if value.Type() == byteArrayType {
		o.writeByteArray(w, value)
	} else if objectType.Kind() == reflect.Slice {
		o.writeSliceValue(w, value)
	} else if util.IsTimeValue(value) {
		o.writeTime(w, value)
	} else {
		o.writePrimitiveValue(w, value)
	}
}

func (o *Encoder) writeByteArray(w io.Writer, value reflect.Value) {
	o.WriteIntValue(w, value.Len())
	l, err := w.Write(value.Bytes())
	util.CheckErr(err)
	if l != len(value.Bytes()) {
		panic(fmt.Sprintf("written length does not match: %d vs %d", l, len(value.Bytes())))
	}
}

func (o *Encoder) writeTime(w io.Writer, value reflect.Value) {
	t := value.Interface().(time.Time)
	u := t.UnixMicro()
	o.WriteInt64Value(w, u)
}

func (o *Encoder) writePrimitiveValue(w io.Writer, value reflect.Value) {
	kind := value.Kind()
	v := value.Interface()
	switch kind {
	case reflect.Int:
		o.WriteIntValue(w, v.(int))
	case reflect.Uint:
		o.WriteUintValue(w, v.(uint))
	case reflect.Int64:
		o.WriteInt64Value(w, v.(int64))
	case reflect.Uint64:
		o.WriteUint64Value(w, v.(uint64))
	case reflect.Float64:
		o.WriteFloat64Value(w, v.(float64))
	case reflect.String:
		o.WriteStringValue(w, v.(string))
	default:
		panic(fmt.Sprintf("Unsupported type %s", value.Type().String()))
	}
}

func (o *Encoder) writeMapValue(w io.Writer, value reflect.Value) {
	o.WriteIntValue(w, value.Len())
	for _, k := range value.MapKeys() {
		currentKey := k.Convert(value.Type().Key())
		currentValue := value.MapIndex(currentKey)
		o.writePrimitiveValue(w, currentKey)
		o.storeField(w, currentValue)
	}
}

func (o *Encoder) writeSliceValue(w io.Writer, value reflect.Value) {
	o.WriteIntValue(w, value.Len())
	for i := 0; i < value.Len(); i++ {
		element := value.Index(i)
		o.storeField(w, element)
	}
}

func (o *Encoder) StoreString(w io.Writer, value *string) {
	if o.StoreAssigned(w, value) {
		o.WriteStringValue(w, *value)
	}
}

func (o *Encoder) WriteStringValue(w io.Writer, value string) {
	data := []byte(value)
	o.WriteUint64Value(w, uint64(len(data)))
	o.write(w, data)
}

func NewEncoder() *Encoder {
	return &Encoder{
		buf64:      make([]byte, 8),
		assigned:   []byte{Assigned},
		unassigned: []byte{NotAssigned},
	}
}
