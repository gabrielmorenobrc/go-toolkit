package prof

import (
	"bytes"
	"fmt"
	"io"
	"sparrowhawktech/toolkit/util"
	"sync"
	"time"
)

var GlobalLatencies *Latencies

type Latencies struct {
	mux       *sync.Mutex
	times     []*time.Time
	averages  []*time.Duration
	counts    []int64
	last      []*time.Duration
	nameMap   map[int]string
	handleMap map[string]int
}

// RegisterName if you mind about usage of string hash keys performance introducing noise, use this and then use the int-based methods
func (o *Latencies) RegisterName(name string) int {
	o.mux.Lock()
	defer o.mux.Unlock()
	i := o.doRegisterName(name)
	return i
}

func (o *Latencies) doRegisterName(name string) int {
	i := len(o.times)
	o.nameMap[i] = name
	o.handleMap[name] = i
	o.times = append(o.times, nil)
	o.averages = append(o.averages, nil)
	o.counts = append(o.counts, 0)
	o.last = append(o.last, nil)
	return i
}

func (o *Latencies) Start(handle int) {
	t := time.Now()
	o.mux.Lock()
	defer o.mux.Unlock()
	o.times[handle] = &t
}

func (o *Latencies) End(handle int) time.Duration {
	t1 := time.Now()
	o.mux.Lock()
	defer o.mux.Unlock()
	d := o.doEnd(handle, t1)
	return d
}

func (o *Latencies) doEnd(handle int, t1 time.Time) time.Duration {
	t0 := o.times[handle]
	d := t1.Sub(*t0)
	avg := o.averages[handle]
	count := o.counts[handle]
	newCount := count + 1
	if count > 0 {
		a := (*avg + d) / time.Duration(newCount)
		o.averages[handle] = &a
	} else {
		o.averages[handle] = &d
	}
	o.counts[handle] = newCount
	o.last[handle] = &d
	return d
}

// StartN if you don't mind about string hash keys performance, you can use this form all the way
func (o *Latencies) StartN(name string) int {
	t := time.Now()
	o.mux.Lock()
	defer o.mux.Unlock()
	handle, ok := o.handleMap[name]
	if !ok {
		handle = o.doRegisterName(name)
	}
	o.times[handle] = &t
	return handle
}

func (o *Latencies) EndN(name string) {
	t := time.Now()
	o.mux.Lock()
	defer o.mux.Unlock()
	handle := o.handleMap[name]
	o.doEnd(handle, t)
}

func (o *Latencies) WriteTo(handle int, w io.Writer) {
	avg := o.averages[handle]
	count := o.counts[handle]
	last := o.last[handle]

	name := o.nameMap[handle]
	b := bytes.Buffer{}
	util.WriteString(name, &b)
	util.WriteString(": last: ", &b)
	if last == nil {
		util.WriteString(" ", &b)
	} else {
		util.WriteString(fmt.Sprintf("%v", *last), &b)
	}
	util.WriteString(fmt.Sprintf(" - count: %d", count), &b)
	util.WriteString(" - avg: ", &b)
	if avg == nil {
		util.WriteString(" ", &b)
	} else {
		util.WriteString(fmt.Sprintf("%v", *avg), &b)
	}
	util.WriteString("\n", &b)
	_, err := io.Copy(w, &b)
	util.CheckErr(err)
}

func (o *Latencies) WriteAllTo(w io.Writer) {
	for _, h := range o.handleMap {
		o.WriteTo(h, w)
	}
}

func (o *Latencies) PrintAll() string {
	b := bytes.Buffer{}
	o.WriteAllTo(&b)
	return b.String()
}

func NewLatencies() *Latencies {
	return &Latencies{
		mux:       &sync.Mutex{},
		times:     nil,
		averages:  nil,
		counts:    nil,
		last:      nil,
		nameMap:   make(map[int]string),
		handleMap: make(map[string]int),
	}
}
func InitGlobalLatencies() {
	GlobalLatencies = NewLatencies()
}
