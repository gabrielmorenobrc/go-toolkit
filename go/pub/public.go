package pub

import (
	"reflect"
	"sparrowhawktech/toolkit/util"
)

var mapTemplate = make(map[string]interface{})

func ToPublic(i interface{}) interface{} {
	if i == nil {
		return nil
	}
	t := reflect.TypeOf(i)
	value := reflect.ValueOf(i)
	if util.IsStructPtr(value) && !value.IsZero() {
		return structToPublic(value.Elem())
	} else if util.IsStructValue(value) {
		return structToPublic(value)
	} else if util.IsArray(t) {
		return arrayToPublic(i)
	} else if value.Kind() == reflect.Map {
		return mapToPublic(i)
	} else {
		return i
	}
}

func structToPublic(value reflect.Value) map[string]interface{} {
	output := make(map[string]interface{})
	t := value.Type()
	for i := 0; i < t.NumField(); i++ {
		fd := t.Field(i)
		f := value.Field(i)
		label := ResolveLabel(fd)
		if f.Interface() != nil {
			output[label] = ToPublic(f.Interface())
		}
	}
	return output
}

func arrayToPublic(i interface{}) []interface{} {
	value := reflect.ValueOf(i)
	l := value.Len()
	result := make([]interface{}, l)
	if l == 0 {
		return result
	}
	for i := 0; i < l; i++ {
		e := value.Index(i)
		v := reflect.Indirect(e)
		m := ToPublic(v.Interface())
		result[i] = m
	}
	return result
}

func mapToPublic(i interface{}) interface{} {
	value := reflect.ValueOf(i)
	result := reflect.MakeMap(reflect.MapOf(value.Type().Key(), reflect.TypeOf(mapTemplate).Elem()))
	for _, k := range value.MapKeys() {
		e := value.MapIndex(k)
		t := reflect.Indirect(e)
		m := ToPublic(t.Interface())
		result.SetMapIndex(k, reflect.ValueOf(m))
	}
	return result.Interface()
}

func ResolveLabel(fd reflect.StructField) string {
	if tagValue, ok := fd.Tag.Lookup("pub"); ok {
		return tagValue
	} else if jsonName, ok := fd.Tag.Lookup("json"); ok {
		return jsonName
	} else {
		n := fd.Name
		return n
	}
}

func ResolveDescription(fd reflect.StructField) string {
	if tagValue, ok := fd.Tag.Lookup("desc"); ok {
		return tagValue
	} else {
		return ResolveLabel(fd)
	}
}
