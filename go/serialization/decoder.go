package serialization

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"time"
)

type Decoder struct {
	buf64  []byte
	buf8   []byte
	buffer *bytes.Buffer
}

func (o *Decoder) ReadFloat64Value(r io.Reader) float64 {
	u := o.ReadUint64Value(r)
	f := math.Float64frombits(u)
	return f
}

func (o *Decoder) ReadUint64Value(r io.Reader) uint64 {
	util.Read(r, o.buf64)
	return binary.BigEndian.Uint64(o.buf64)
}

func (o *Decoder) ReadInt64Value(r io.Reader) int64 {
	util.Read(r, o.buf64)
	return int64(binary.BigEndian.Uint64(o.buf64))
}

func (o *Decoder) ReadUint32Value(r io.Reader) uint32 {
	util.Read(r, o.buf64)
	return uint32(binary.BigEndian.Uint64(o.buf64))
}

func (o *Decoder) ReadInt32Value(r io.Reader) int32 {
	util.Read(r, o.buf64)
	return int32(binary.BigEndian.Uint64(o.buf64))
}

func (o *Decoder) ReadIntValue(r io.Reader) int {
	util.Read(r, o.buf64)
	return int(binary.BigEndian.Uint64(o.buf64))
}

func (o *Decoder) LoadUint64(r io.Reader) *uint64 {
	if o.ReadAssigned(r) {
		return util.PUint64(o.ReadUint64Value(r))
	} else {
		return nil
	}
}

func (o *Decoder) LoadUint32(r io.Reader) *uint32 {
	if o.ReadAssigned(r) {
		return util.PUint32(o.ReadUint32Value(r))
	} else {
		return nil
	}
}

func (o *Decoder) LoadInt32(r io.Reader) *int32 {
	if o.ReadAssigned(r) {
		i32 := o.ReadInt32Value(r)
		return &i32
	} else {
		return nil
	}
}

func (o *Decoder) LoadBool(r io.Reader) *bool {
	if o.ReadAssigned(r) {
		util.Read(r, o.buf8)
		return util.PBool(o.buf8[0] == 1)
	} else {
		return nil
	}
}

func (o *Decoder) LoadInt(r io.Reader) *int {
	if o.ReadAssigned(r) {
		u64 := o.ReadUint64Value(r)
		i := int(u64)
		return &i
	} else {
		return nil
	}
}

func (o *Decoder) LoadString(r io.Reader) *string {
	if o.ReadAssigned(r) {
		s := o.ReadStringValue(r)
		return &s
	} else {
		return nil
	}
}

func (o *Decoder) ReadStringValue(r io.Reader) string {
	l := o.ReadUint64Value(r)
	buffer := o.read(r, int(l))
	return string(buffer)
}

func (o *Decoder) LoadBytes(r io.Reader) []byte {
	if o.ReadAssigned(r) {
		l := o.ReadUint64Value(r)
		buffer := o.read(r, int(l))
		return buffer
	} else {
		return nil
	}
}

func (o *Decoder) LoadU32Slice(r io.Reader) []uint32 {
	if o.ReadAssigned(r) {
		sizeToRead := o.ReadUint64Value(r)
		data := make([]uint32, sizeToRead)
		for i := range data {
			data[i] = o.ReadUint32Value(r)
		}
		return data
	} else {
		return nil
	}
}

func (o *Decoder) ReadAssigned(r io.Reader) bool {
	util.Read(r, o.buf8)
	return o.buf8[0] == Assigned
}

func (o *Decoder) read(r io.Reader, size int) []byte {
	o.buffer.Reset()
	_, err := io.CopyN(o.buffer, r, int64(size))
	util.CheckErr(err)
	return o.buffer.Bytes()
}

func (o *Decoder) readTime(r io.Reader) time.Time {
	u := o.ReadInt64Value(r)
	t := time.UnixMicro(u)
	return t
}

func (o *Decoder) LoadStruct(r io.Reader, s interface{}) bool {
	value := reflect.ValueOf(s)
	if o.ReadAssigned(r) {
		o.loadStructValue(r, value.Elem())
		return true
	} else {
		return false
	}
}

func (o *Decoder) loadValue(r io.Reader, value reflect.Value) bool {
	if !o.ReadAssigned(r) {
		return false
	}

	valueType := value.Type()
	if valueType.Kind() == reflect.Ptr {
		valueType = valueType.Elem()
	}
	v := o.readValue(r, valueType)
	rv := reflect.ValueOf(v)
	k := value.Type().Kind()
	if k == reflect.Ptr || k == reflect.Slice || k == reflect.Map {
		value.Set(rv)
	} else {
		value.Set(rv.Elem())
	}
	return true
}

func (o *Decoder) readValue(r io.Reader, fieldType reflect.Type) interface{} {
	if util.IsStruct(fieldType) {
		s := reflect.New(fieldType)
		o.loadStructValue(r, s.Elem())
		return s.Interface()
	} else if fieldType.Kind() == reflect.Map {
		return o.readMapValue(r, fieldType)
	} else if fieldType == byteArrayType {
		return o.readByteArray(r)
	} else if fieldType.Kind() == reflect.Slice {
		return o.readSliceValue(r, fieldType)
	} else if util.IsTime(fieldType) {
		t := o.readTime(r)
		return &t
	} else {
		return o.readPrimitiveValue(r, fieldType)
	}
}

func (o *Decoder) readByteArray(r io.Reader) interface{} {
	l := o.ReadIntValue(r)
	result := o.read(r, l)
	return result
}

func (o *Decoder) loadStructValue(r io.Reader, value reflect.Value) {
	objectType := value.Type()
	for i := 0; i < objectType.NumField(); i++ {
		field := objectType.Field(i)
		if !field.IsExported() {
			continue
		}
		f := value.FieldByName(field.Name)
		o.loadValue(r, f)
	}
}

func (o *Decoder) readPrimitiveValue(r io.Reader, valueType reflect.Type) interface{} {
	kind := valueType.Kind()
	switch kind {
	case reflect.Int:
		v := o.ReadIntValue(r)
		return &v
	case reflect.Uint:
		v := uint(o.ReadUint64Value(r))
		return &v
	case reflect.Int64:
		v := o.ReadInt64Value(r)
		return &v
	case reflect.Uint64:
		v := o.ReadUint64Value(r)
		return &v
	case reflect.Float64:
		v := o.ReadFloat64Value(r)
		return &v
	case reflect.String:
		v := o.ReadStringValue(r)
		return &v
	default:
		panic(fmt.Sprintf("Unsupported type %s", valueType.String()))
	}
}

func (o *Decoder) readSliceValue(r io.Reader, sliceType reflect.Type) interface{} {
	count := o.ReadIntValue(r)
	result := reflect.MakeSlice(sliceType, count, count)
	valueType := sliceType.Elem()
	for i := 0; i < count; i++ {
		if o.ReadAssigned(r) {
			v := reflect.ValueOf(o.readValue(r, valueType))
			if valueType.Kind() == reflect.Ptr {
				result.Index(i).Set(v)
			} else {
				result.Index(i).Set(reflect.Indirect(v))
			}
		}
	}
	return result.Interface()
}

func (o *Decoder) readMapValue(r io.Reader, mapType reflect.Type) interface{} {
	result := reflect.MakeMap(mapType)
	count := o.ReadIntValue(r)
	keyType := mapType.Key()
	valueType := mapType.Elem()
	for n := 0; n < count; n++ {
		k := reflect.Indirect(reflect.ValueOf(o.readPrimitiveValue(r, keyType)))
		if o.ReadAssigned(r) {
			v := reflect.ValueOf(o.readValue(r, valueType))
			if valueType.Kind() == reflect.Ptr {
				result.SetMapIndex(k, v)
			} else {
				result.SetMapIndex(k, reflect.Indirect(v))
			}
		}
	}
	return result.Interface()
}

func NewDecoder() *Decoder {
	return &Decoder{
		buf64:  make([]byte, 8),
		buf8:   make([]byte, 1),
		buffer: &bytes.Buffer{},
	}
}
