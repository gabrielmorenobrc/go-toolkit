package serialization_test

import (
	"bytes"
	"fmt"
	"os"
	"sparrowhawktech/toolkit/util"
	"testing"
	"time"

	"toolkit/serialization"
)

type B struct {
	S *string
	I *int64
	F *float64
}

type A struct {
	V          string
	S          *string
	I          *int64
	F          *float64
	T          *time.Time
	B          *B
	BSlice     []B
	BStringMap map[string]B
	BIntMap    map[int]B
}

func TestDeep(t *testing.T) {
	a := A{
		V: "hi mom",
		S: util.PStr("A string"),
		I: util.PInt64(1),
		F: util.PFloat64(1.0),
		T: util.PTime(time.Now()),
		B: &B{
			S: util.PStr("B string"),
			I: nil,
			F: nil,
		},
		BSlice: []B{{
			S: util.PStr("B[0] string"),
			I: util.PInt64(0),
			F: util.PFloat64(0.1),
		}, {
			S: util.PStr("B[1] string"),
			I: util.PInt64(1),
			F: util.PFloat64(0.2),
		}},
		BStringMap: map[string]B{
			"B.a": {
				S: util.PStr("B.a string"),
				I: util.PInt64(1),
				F: util.PFloat64(1.1),
			},
			"B.b": {
				S: util.PStr("B.b string"),
				I: util.PInt64(2),
				F: util.PFloat64(1.2),
			},
		},
		BIntMap: map[int]B{
			1: {
				S: util.PStr("B.1 string"),
				I: util.PInt64(1),
				F: util.PFloat64(1.1),
			},
			2: {
				S: util.PStr("B.2 string"),
				I: util.PInt64(2),
				F: util.PFloat64(1.2),
			},
		},
	}
	before := util.MarshalPretty(a)
	os.Stdout.Write(before)

	buffer := bytes.Buffer{}

	encoder := serialization.NewEncoder()
	t0 := time.Now()
	encoder.StoreStruct(&buffer, a)
	t1 := time.Now()

	fmt.Printf("Store: %v\n", t1.Sub(t0))

	decoder := serialization.NewDecoder()
	a = A{}
	t0 = time.Now()
	decoder.LoadStruct(&buffer, &a)
	t1 = time.Now()
	fmt.Printf("Load: %v\n", t1.Sub(t0))

	//runtime.GC()
	time.Sleep(time.Second * 2)

}
