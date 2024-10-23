package storage_test

import (
	"fmt"
	"os"
	"sparrowhawktech/toolkit/util"
	"testing"
	"time"
	"toolkit/serialization"
	"toolkit/storage"
)

const sixMonths = 24 * 180

type E struct {
	A *string
	B *int
}

func TestFileStore(t *testing.T) {
	//util.SetDefaultStackTraceTag("error")
	const fileName = "store-test.dat"
	cleanup(fileName)
	defer func() {
		cleanup(fileName)
	}()
	doIt(fileName)
}

func TestCorrupted(t *testing.T) {
	util.SetDefaultStackTraceTag("error")
	const fileName = "corrupt-test.dat"
	cleanup(fileName)
	defer func() {
		cleanup(fileName)
	}()

	store := storage.NewFileStore(fileName)
	store.Open()
	defer store.Close()
	populate(store)
	store.Close()
	store.Open()
	store.InsertCorrupted()

	count := store.Count()
	if count != sixMonths+1 {
		panic(fmt.Sprintf("Unexpected store count: %d", count))
	}
	println("count", count)

	store.Vacuum(sixMonths) //TODO: there is still something wrong with corrupted rows and the count here
	count = store.Count()
	if count != sixMonths-1 {
		panic(fmt.Sprintf("Unexpected store count: %d", count))
	}
	println("count", count)

	read(store, 1)
}

func cleanup(fileName string) {
	if util.FileExists(fileName) {
		err := os.Remove(fileName)
		util.CheckErr(err)
	}
}

func doIt(fileName string) {
	store := storage.NewFileStore(fileName)
	store.Open()
	defer store.Close()
	populate(store)
	read(store, 0)
	store.Vacuum(10)
	read(store, sixMonths-10)
	count := store.Count()
	if count != 10 {
		panic(fmt.Sprintf("Unexpected store count: %d", count))
	}
	println("count", count)

	ch := make(chan bool)
	go func() {
		populate(store)
		ch <- true
	}()
	go func() {
		read(store, -1)
		ch <- true
	}()
	go func() {
		store.Vacuum(sixMonths)
		ch <- true
	}()

	for n := 0; n < 3; n++ {
		select {
		case <-ch:
			continue
		case <-time.After(time.Second):
			panic("timeout")
		}
	}

	println("final count", store.Count())
}

func read(store *storage.FileStore, startAt int) {
	decoder := serialization.NewDecoder()
	t0 := time.Now()
	count := store.StartRead()
	defer store.EndRead()
	n := 0
	i := 0
	println("read count", count)
	for i = 0; i < count; i++ {
		e := E{}
		r := store.ReadNext()
		decoder.LoadStruct(r, &e)
		if startAt > -1 && *e.B != int(i+startAt) {
			panic(fmt.Sprintf("%d != %d", *e.B, i+startAt))
		}
		n++
		if n == 1000 {
			println(*e.A)
			n = 0
		}
	}
	println(i)

	t1 := time.Now()

	fmt.Printf("%v\n", t1.Sub(t0))

}

func populate(store *storage.FileStore) {
	for i := 0; i < sixMonths; i++ {
		insertRow(store, i)
	}
}

func insertRow(store *storage.FileStore, i int) {
	e := E{
		A: util.PStrf("%d", i),
		B: util.PInt(i),
	}
	w := store.StartRow()
	defer store.EndRow()
	serialization.NewEncoder().StoreStruct(w, e)
}
