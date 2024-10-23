package util

import (
	"fmt"
	"reflect"
	"sparrowhawktech/toolkit/util"
)

func ValidateStruct(s interface{}) {
	doValidateStruct(s, "")
}

func doValidateStruct(s interface{}, path string) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fd := t.Field(i)
		if fd.IsExported() {
			f := v.Field(i)
			tv, ok := fd.Tag.Lookup("require")
			if ok && tv == "true" && f.IsNil() {
				panic(fmt.Sprintf("%s.%s", path, fd.Name))
			}
			if util.IsStructPtr(f) && !f.IsZero() {
				doValidateStruct(f.Elem().Interface(), path+"."+fd.Name)
			}
		}
	}
}
