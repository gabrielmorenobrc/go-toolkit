package dispatch_test

import (
	"fmt"
	"os"
	"sparrowhawktech/toolkit/util"
	"testing"
	"time"
	"toolkit/dispatch"
)

func TestDispatcher(t *testing.T) {
	util.SetDefaultStackTraceTag("error")
	run(1, time.Millisecond*100)
	run(1, time.Second)
	run(2, time.Second)
	run(100, time.Millisecond*100)
}

func run(eventCount int, minFrequency time.Duration) {
	dispatcher := dispatch.NewDispatcher()

	ch := make(chan int)
	l1 := func(topic string, i interface{}) {
		defer func() { ch <- i.(int) }()
		util.Log("info").Printf("l1: %v", i)
		time.Sleep(minFrequency + time.Millisecond)
	}

	l2 := func(topic string, i interface{}) {
		defer func() { ch <- i.(int) }()
		util.Log("info").Printf("l2: %v", i)
	}

	t1 := "topic1"
	dispatcher.RegisterListener(l1, t1)
	dispatcher.RegisterListener(l2, t1)

	go func() {
		for i := 0; i < eventCount-1; i++ {
			go dispatchEvent(dispatcher, t1, i+1, minFrequency)
		}
		time.Sleep(minFrequency)
		go dispatchEvent(dispatcher, t1, eventCount, minFrequency)
	}()

	defer func() {
		report := dispatcher.Report()
		util.JsonPretty(report, os.Stdout)
		reportItem := report[0]
		dispatchCount := reportItem.TopicInfo.DispatchCount
		if dispatchCount != int64(eventCount) {
			panic(fmt.Sprintf("Dispatch count != %d but = %d", eventCount, dispatchCount))
		}
		if reportItem.TopicInfo.EffectiveDispatchCount == 0 {
			panic("Effective dispatch count is 0!")
		}
	}()
	waitForLast(ch, eventCount, minFrequency)
}

func waitForLast(ch chan int, eventCount int, minFrequency time.Duration) {
	n := 0
	for {
		select {
		case i := <-ch:
			if i == eventCount {
				n++
			}
			if n == 2 {
				return
			}
		case <-time.After(minFrequency * time.Duration(eventCount) * 2):
			panic("timeout")
		}
	}
}

func dispatchEvent(dispatcher *dispatch.Dispatcher, t string, i int, minFrequency time.Duration) {
	dispatcher.DispatchWithThrottling(t, minFrequency, i)
}
