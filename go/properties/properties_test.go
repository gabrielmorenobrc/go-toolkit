package properties_test

import (
	"fmt"
	"os"
	"sparrowhawktech/toolkit/util"
	"sync"
	"testing"
	"toolkit/properties"
)

type A struct {
	Index int
	AM    map[string]string
}

type B struct {
	Index       int
	PIndex      *int
	A           *A
	BM          map[string]string
	Slice       []int
	StructSlice []*A
	StructMap   map[string]*A
}

func TestSimple(t *testing.T) {
	master := &B{
		Index:       -1,
		PIndex:      util.PInt(-1),
		A:           &A{AM: make(map[string]string)},
		BM:          make(map[string]string),
		Slice:       []int{1, 2, 3},
		StructSlice: []*A{{Index: -1}, nil},
		StructMap:   map[string]*A{"notNil": {Index: -1}, "nil": nil},
	}
	master.BM["foo"] = "bar"
	master.A.AM["bar"] = "foo"
	count := 100000
	p := properties.SimpleProperty[B]{}
	p.Set(master)
	all := run(count, &p)

	util.JsonPretty(master, os.Stdout)
	if master.Index != -1 {
		panic("b.Index changed")
	} else if *master.PIndex != -1 {
		panic("b.PIndex changed")
	} else if master.A.AM["bar"] != "foo" {
		panic("b.a.AM changed")
	} else if master.BM["foo"] != "bar" {
		panic("b.BM changed")
	}

	maxIndex := 0

	for _, b := range all {

		if b.Index == 0 {
			panic("Index is 0!")
		}

		s := fmt.Sprintf("%d", b.Index)
		if v, ok := b.BM[s]; !ok {
			panic("b.BM[s] not found")
		} else if v != s {
			panic("b.BM[s] does not match")
		}
		if len(b.BM) != 2 {
			panic(fmt.Sprintf("len of b.BM is %d", len(b.BM)))
		}
		if v, ok := b.A.AM[s]; !ok {
			panic("b.A.AM[s] not found")
		} else if v != s {
			panic("b.A.AM[s] does not match")
		}
		if len(b.A.AM) != 2 {
			panic(fmt.Sprintf("len of b.A.AM is %d", len(b.BM)))
		}
		if len(b.Slice) != 3 {
			panic("Something funny with the Slice, count differs from expected")
		}
		if b.StructSlice[0].Index != b.Index {
			panic("Struct pointer in slice seems to be messed up")
		}
		if b.StructMap["notNil"].Index != b.Index {
			panic("Struct pointer in map seems to be messed up")
		}
		if b.StructSlice[1] != nil {
			panic("Slice item is not nil as expected")
		}
		if b.StructMap["nil"] != nil {
			panic("Map item is not nil as expected")
		}
		if maxIndex < b.Index {
			maxIndex = b.Index
		}
	}

	util.Log("info").Printf("Max index: %d", maxIndex)
	if maxIndex != len(all) {
		panic("Max index differs from expected")
	}

}

func run(count int, p *properties.SimpleProperty[B]) []B {
	ch := make(chan B)
	all := make([]B, count)
	go listen(ch, all)
	defer close(ch)
	wg := &sync.WaitGroup{}
	for i := 0; i < len(all); i++ {
		wg.Add(1)
		go messAround(p.Snapshot(), i+1, wg, ch)
	}
	wg.Wait()
	return all
}

func listen(ch chan B, all []B) {
	m := sync.Mutex{}
	f := func(all []B, b B) {
		m.Lock()
		defer m.Unlock()
		all[b.Index-1] = b
	}
	for {
		b, open := <-ch
		if open {
			f(all, b)
		} else {
			break
		}
	}
}

func messAround(b *B, i int, wg *sync.WaitGroup, ch chan B) {
	defer wg.Done()
	b.Index = i
	b.PIndex = &i
	s := fmt.Sprintf("%d", i)
	b.BM[s] = s
	b.A.AM[s] = s
	b.StructSlice[0].Index = i
	b.StructMap["notNil"].Index = i
	ch <- *b
}
