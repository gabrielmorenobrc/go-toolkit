package util

import (
	"os"
	"os/signal"
	"runtime"
	"sparrowhawktech/toolkit/util"
	"sync"
	"syscall"
	"time"
)

type ShutdownHook func(signal os.Signal)

type ShutdownHookEntry struct {
	Label   *string
	Hook    ShutdownHook
	Timeout *time.Duration
}

type ShutdownHooks struct {
	mux     *sync.Mutex
	hooks   []ShutdownHookEntry
	signals []os.Signal
}

func (o *ShutdownHooks) AddWithTimeout(label string, f ShutdownHook, timeout time.Duration) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.hooks = append(o.hooks, ShutdownHookEntry{Label: &label, Hook: f, Timeout: &timeout})
}

func (o *ShutdownHooks) Add(label string, f ShutdownHook) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.hooks = append(o.hooks, ShutdownHookEntry{Label: &label, Hook: f, Timeout: nil})
}

func (o *ShutdownHooks) Init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, o.signals...)
	sig := <-c
	signal.Reset(sig)
	if runtime.GOOS != "windows" {
		defer func() {
			p, err := os.FindProcess(os.Getpid())
			util.CheckErr(err)
			err = p.Signal(sig)
			util.CheckErr(err)
		}()
	}
	util.Log("info").Printf("Processing shutdown hooks. SIGNAL: %s", sig.String())
	o.processHooks(sig)
}

func (o *ShutdownHooks) processHooks(s os.Signal) {
	wg := sync.WaitGroup{}
	wg.Add(len(o.hooks))
	timeout := time.Second * 0
	for _, h := range o.hooks {
		if h.Timeout != nil && *h.Timeout > timeout {
			timeout = *h.Timeout
		}
		go o.processHook(&wg, h, s)
	}
	if timeout == time.Second*0 {
		timeout = DefaultShutdownHookTimeout
	}
	go o.timeout(timeout)
	wg.Wait()
}

func (o *ShutdownHooks) processHook(wg *sync.WaitGroup, h ShutdownHookEntry, s os.Signal) {
	defer wg.Done()
	defer util.CatchPanic()
	util.Log("info").Printf("Processing shutdown hook %s\n", *h.Label)
	h.Hook(s)
	util.Log("info").Printf("Finished shutdown hook %s\n", *h.Label)

}

func (o *ShutdownHooks) timeout(timeout time.Duration) {
	time.Sleep(timeout)
	panic("Timed out waiting for shutdown hooks to execute.")
}

func NewShutdownHooks(s ...os.Signal) *ShutdownHooks {
	r := ShutdownHooks{
		mux:     &sync.Mutex{},
		signals: s,
		hooks:   make([]ShutdownHookEntry, 0),
	}
	return &r
}

var DefaultShutdownHooks *ShutdownHooks
var DefaultShutdownHookTimeout = time.Second

func init() {
	DefaultShutdownHooks = NewShutdownHooks(os.Kill, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGABRT)
	go DefaultShutdownHooks.Init()
}
