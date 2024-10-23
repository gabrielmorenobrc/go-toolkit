package pub

import (
	"reflect"
	"sparrowhawktech/toolkit/util"
)

func Describe(a any) DiffNode {
	t := reflect.TypeOf(a)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	keyName := t.Name()
	root := DiffNode{KeyName: &keyName}
	describeNode(t, &root)
	return root
}

func describeNode(t1 reflect.Type, thisNode *DiffNode) {
	for i := 0; i < t1.NumField(); i++ {
		ft := t1.Field(i)
		if !ft.IsExported() {
			continue
		}
		t := ft.Type
		if t.Kind() == reflect.Map {
			t = t.Elem()
		} else if t.Kind() == reflect.Slice {
			t = t.Elem()
		} else if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		label1 := ResolveLabel(ft)
		oldName := label1
		child := &DiffNode{
			KeyName:    &ft.Name,
			PublicName: &oldName,
		}
		if util.IsStruct(t) {
			describeNode(t, child)
		}
		thisNode.Children = append(thisNode.Children, child)

	}
}
