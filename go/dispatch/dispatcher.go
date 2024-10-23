package dispatch

import (
	"fmt"
	"reflect"
	"sparrowhawktech/toolkit/util"
	"sync"
	"sync/atomic"
	"time"
)

const defaultMinFrequency = 0

var GlobalDispatcher = NewDispatcher()

type Listener = func(topic string, i interface{})

type listenerEntry struct {
	listener Listener
}

type TopicInfo struct {
	FirstTime              *time.Time
	LastTime               *time.Time
	Latency                *time.Duration
	AccumLatency           time.Duration
	DispatchCount          int64
	EffectiveDispatchCount int64
	AcquireTime            *time.Time
	ReleaseTime            *time.Time
	LastMinFrequency       *time.Duration
	MaxQueueSize           int
}

type eventEntry struct {
	listeners []listenerEntry
	data      interface{}
}

type eventQueue struct {
	mux     *sync.Mutex
	entries []eventEntry
}

func (o *eventQueue) Flush() []eventEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	result := o.entries
	o.entries = make([]eventEntry, 0)
	return result
}

func (o *eventQueue) Post(data interface{}, listeners []listenerEntry) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.entries = append(o.entries, eventEntry{
		listeners: listeners,
		data:      data,
	})
}

type topicEntry struct {
	mux       *sync.Mutex
	topicInfo atomic.Pointer[TopicInfo]
}

func (o *topicEntry) Acquire() {
	o.mux.Lock()
	defer func() {
		if r := recover(); r != nil {
			o.mux.Unlock()
			panic(r)
		}
	}()
	info := *o.topicInfo.Load()
	info.AcquireTime = util.PTime(time.Now())
	info.ReleaseTime = nil
	o.topicInfo.Store(&info)
}

func (o *topicEntry) Release() {
	defer o.mux.Unlock()
	info := *o.topicInfo.Load()
	info.ReleaseTime = util.PTime(time.Now())
	o.topicInfo.Store(&info)
}

type ReportItem struct {
	TopicName              *string
	TopicInfo              *TopicInfo
	LastMinFrequecyInUnits *string
}

type Dispatcher struct {
	listeners    map[string][]listenerEntry
	topics       map[string]*topicEntry
	queues       map[string]*eventQueue
	listenersMux *sync.Mutex
	topicsMux    *sync.Mutex
	queuesMux    *sync.Mutex
}

func (o *Dispatcher) RegisterListener(l Listener, topics ...string) {
	o.listenersMux.Lock()
	defer o.listenersMux.Unlock()
	for _, topic := range topics {
		list, ok := o.listeners[topic]
		if !ok {
			list = make([]listenerEntry, 1)
			list[0] = listenerEntry{listener: l}
			o.listeners[topic] = list
		} else {
			if resolveEntryIndex(list, l) == nil {
				o.listeners[topic] = append(list, listenerEntry{listener: l})
			} else {
				panic(fmt.Sprintf("Listener already registered for topic %s", topic))
			}
		}
	}
}

func resolveEntryIndex(list []listenerEntry, l Listener) *int {
	for i, elem := range list {
		if reflect.ValueOf(elem.listener).Pointer() == reflect.ValueOf(l).Pointer() {
			return &i
		}
	}
	return nil
}

func (o *Dispatcher) UnregisterListener(l Listener, topics ...string) {
	o.listenersMux.Lock()
	defer o.listenersMux.Unlock()
	for _, topic := range topics {
		list, ok := o.listeners[topic]
		if ok {
			i := resolveEntryIndex(list, l)
			if i != nil {
				//order is not important
				list[*i] = list[len(list)-1]
				list = list[:len(list)-1]
			}
		}
		o.listeners[topic] = list
	}
}

func (o *Dispatcher) listenerSnapshot(topic string) []listenerEntry {
	o.listenersMux.Lock()
	defer o.listenersMux.Unlock()
	if list, ok := o.listeners[topic]; ok {
		return list
	} else {
		return nil
	}
}

func (o *Dispatcher) DispatchWithThrottling(topic string, minFrequency time.Duration, i interface{}) {
	o.dispatch(topic, minFrequency, i)
}

func (o *Dispatcher) Dispatch(topic string, i interface{}) {
	o.dispatch(topic, defaultMinFrequency, i)
}

func (o *Dispatcher) dispatch(topic string, minFrequency time.Duration, i interface{}) {
	listeners := o.listenerSnapshot(topic)
	queue := o.findQueue(topic)
	queue.Post(i, listeners)
	go o.checkQueue(topic, minFrequency, queue)
}

func (o *Dispatcher) checkQueue(topic string, minFrequency time.Duration, queue *eventQueue) {
	defer util.CatchPanic()
	topicEntry := o.findTopic(topic)
	topicEntry.Acquire()
	defer topicEntry.Release()
	info := *topicEntry.topicInfo.Load()
	if info.FirstTime == nil {
		t0 := time.Now()
		info.FirstTime = &t0
	}
	info.LastMinFrequency = &minFrequency
	topicEntry.topicInfo.Store(&info)
	o.processQueue(topic, topicEntry, queue, minFrequency)
}

func (o *Dispatcher) processQueue(topic string, topicEntry *topicEntry, queue *eventQueue, minFrequency time.Duration) {
	events := queue.Flush()
	for len(events) > 0 {
		o.processEvents(topic, topicEntry, events, minFrequency)
		events = queue.Flush()
	}
}

func (o *Dispatcher) processEvents(topic string, topicEntry *topicEntry, events []eventEntry, minFrequency time.Duration) {
	info := *topicEntry.topicInfo.Load()
	l := len(events)
	if info.MaxQueueSize < l {
		info.MaxQueueSize = l
	}
	info.LastMinFrequency = &minFrequency
	topicEntry.topicInfo.Store(&info)
	for i := 0; i < l-1; i++ {
		e := events[i]
		o.processEvent(topic, topicEntry, e, false)
	}
	// Ensure always the latest one is processed, plain throttling logic could skip processing it
	e := events[len(events)-1]
	info = *topicEntry.topicInfo.Load()
	delta := time.Duration(0)
	if info.LastTime != nil {
		delta = info.LastTime.Sub(time.Now())
	}
	/**
	  	  if info.LastTime != nil {
	  	  	delta = time.Now().Sub(*info.LastTime)
	  	  }
	        delta = minFrequency - delta

	*/

	if delta > 0 {
		time.Sleep(delta)
	}

	o.processEvent(topic, topicEntry, e, true)
}

func (o *Dispatcher) processEvent(topic string, topicEntry *topicEntry, e eventEntry, force bool) bool {
	info := *topicEntry.topicInfo.Load()
	info.DispatchCount++
	t0 := time.Now()
	process := force || info.LastTime == nil || t0.Sub(*info.LastTime) >= *info.LastMinFrequency
	if process {
		info.EffectiveDispatchCount++
		topicEntry.topicInfo.Store(&info)
		o.doDispatch(topic, e.listeners, e.data)
		t1 := time.Now()
		info.LastTime = &t1
		latency := t1.Sub(t0)
		info.Latency = &latency
		info.AccumLatency += *info.Latency
	}
	topicEntry.topicInfo.Store(&info)
	return process
}

func (o *Dispatcher) doDispatch(topic string, list []listenerEntry, i interface{}) {
	w := sync.WaitGroup{}
	d := 0
	for n, l := range list {
		w.Add(1)
		d++
		go o.notifyAsyncListener(topic, &w, l.listener, i, n)
	}
	if d > 0 {
		w.Wait()
	}
}

func (o *Dispatcher) notifyAsyncListener(topic string, w *sync.WaitGroup, l Listener, i interface{}, n int) {
	defer w.Done()
	defer func() {
		if r := recover(); r != nil {
			util.ProcessError(r)
		}
	}()
	util.Log("dispatch").Printf("Executing listener %d for topic %s", n, topic)
	t0 := time.Now()
	l(topic, i)
	util.Log("dispatch").Printf("Finished executing listener %d for topic %s. Took %v", n, topic, time.Now().Sub(t0))

}

func (o *Dispatcher) findTopic(name string) *topicEntry {
	o.topicsMux.Lock()
	defer o.topicsMux.Unlock()
	entry, ok := o.topics[name]
	if !ok {
		entry = &topicEntry{mux: &sync.Mutex{}}
		entry.topicInfo.Store(&TopicInfo{})
		o.topics[name] = entry
	}
	return entry
}

func (o *Dispatcher) findQueue(topic string) *eventQueue {
	o.queuesMux.Lock()
	defer o.queuesMux.Unlock()
	entry, ok := o.queues[topic]
	if !ok {
		entry = &eventQueue{mux: &sync.Mutex{}, entries: make([]eventEntry, 0)}
		o.queues[topic] = entry
	}
	return entry
}

func (o *Dispatcher) topicsSnapshot() map[string]*topicEntry {
	o.topicsMux.Lock()
	defer o.topicsMux.Unlock()
	result := make(map[string]*topicEntry, len(o.topics))
	for k, t := range o.topics {
		result[k] = t
	}
	return result
}

func (o *Dispatcher) Report() []ReportItem {
	snapshot := o.topicsSnapshot()
	result := make([]ReportItem, len(snapshot))
	i := 0
	for k, t := range snapshot {
		name := k
		info := *t.topicInfo.Load()
		mfs := fmt.Sprintf("%v", *info.LastMinFrequency)
		ri := ReportItem{
			TopicName:              &name,
			TopicInfo:              &info,
			LastMinFrequecyInUnits: &mfs,
		}
		result[i] = ri
		i++
	}
	return result
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		listeners:    make(map[string][]listenerEntry),
		topics:       make(map[string]*topicEntry),
		queues:       make(map[string]*eventQueue),
		listenersMux: &sync.Mutex{},
		topicsMux:    &sync.Mutex{},
		queuesMux:    &sync.Mutex{}}
}
