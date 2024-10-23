package prof

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sparrowhawktech/toolkit/util"
	"sync"
	"sync/atomic"
	"time"
	"toolkit/dispatch"
	util2 "toolkit/util"
)

var DefaultMonitor *Monitor

type Monitor struct {
	interval       *time.Duration
	prefix         *string
	t              *time.Ticker
	maxFileCount   *int
	lastTime       *time.Time
	memProfileOpen atomic.Bool
	cpuProfileOpen atomic.Bool
	cpuTempName    string
	cpuFileName    string
	mux            *sync.Mutex
}

func (o *Monitor) Start() {
	o.mux.Lock()
	defer o.mux.Unlock()
	if o.t != nil {
		util.Log("error").Println("Profile monitor start called but it was already running")
		return
	}
	util.Log("info").Printf("Starting profile monitor [%d / %d]", *o.interval, *o.maxFileCount)
	o.t = time.NewTicker(*o.interval)
	util2.DefaultShutdownHooks.Add("Profiling monitor", func(s os.Signal) {
		o.tearDown()
	})
	o.processTicker()
	now := time.Now()
	o.lastTime = &now
	go o.run()
}

func (o *Monitor) run() {
	for range o.t.C {
		now := time.Now()
		if o.lastTime == nil || now.Sub(*o.lastTime) > *o.interval {
			o.processTicker()
			o.lastTime = &now
		}
	}
}

func (o *Monitor) processTicker() {
	o.printStats()
	go o.startCpuProfile()
	go o.profileMem()
	o.du()
	o.df()
	o.dispatcherReport()
}

func (o *Monitor) printStats() {
	defer util.CatchPanic()
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)
	util.Log("info").Printf("PROFILE: Sys=%d\tHeap alloc=%d\tMallocs=%d\tFrees=%d\tNum GC=%d",
		stats.Sys, stats.HeapAlloc, stats.Mallocs, stats.Frees,
		stats.NumGC)
}

func (o *Monitor) tearDown() {
	o.t.Stop()
	o.t = nil
	o.stopCpuProfile()
	if !o.memProfileOpen.Load() {
		o.profileMem()
	}
}

func (o *Monitor) startCpuProfile() {
	defer util.CatchPanic()
	if o.cpuProfileOpen.Load() {
		o.stopCpuProfile()
	}
	o.cpuProfileOpen.Store(true)
	name := nextFileSpec(*o.prefix+"-cpu.pprof", *o.maxFileCount)
	util.Log("info").Printf("Starting cpu profile log %s", name)
	f, err := os.CreateTemp("./", "cpu.tmp")
	util.CheckErr(err)
	o.cpuTempName = f.Name()
	o.cpuFileName = name
	util.CheckErr(pprof.StartCPUProfile(f))
}

func (o *Monitor) stopCpuProfile() {
	defer util.CatchPanic()
	pprof.StopCPUProfile()

	o.renameTemp(o.cpuTempName, o.cpuFileName)
	o.cpuProfileOpen.Store(false)
}

func (o *Monitor) profileMem() {
	defer util.CatchPanic()
	o.memProfileOpen.Store(true)
	defer func() { o.memProfileOpen.Store(false) }()
	name := nextFileSpec(*o.prefix+"-mem.pprof", *o.maxFileCount)
	util.Log("info").Printf("Creating mem profile snapshot %s", name)
	f, err := os.CreateTemp("./", "mem.tmp")
	util.CheckErr(err)
	defer o.renameTemp(f.Name(), name)
	defer util.Close(f)
	util.CheckErr(pprof.WriteHeapProfile(f))
}

func (o *Monitor) du() {
	defer util.CatchPanic()
	out := util.RunCmd("bash", "-c", "du -h --threshold=10M | sort -h -r | head -n 100")
	util.Log("info").Printf("DU:\n%s\n", out)
}

func (o *Monitor) df() {
	defer util.CatchPanic()
	out := util.RunCmd("df")
	util.Log("info").Printf("DF:\n%s\n", out)
}

func (o *Monitor) Stop() {
	o.mux.Lock()
	defer o.mux.Unlock()
	if o.t == nil {
		util.Log("error").Printf("Profiling stop called but it wasn't running.")
	} else {
		o.tearDown()
	}
}

func (o *Monitor) renameTemp(name1 string, name2 string) {
	util.CheckErr(os.Rename(name1, name2))
}

func (o *Monitor) dispatcherReport() {
	report := dispatch.GlobalDispatcher.Report()
	util.Log("info").Printf("Dispatcher stats:\n%s\n", string(util.MarshalPretty(report)))
}

func NewMonitor(interval time.Duration, maxFileCount int, prefix string) *Monitor {
	return &Monitor{interval: &interval, maxFileCount: &maxFileCount, prefix: &prefix, mux: &sync.Mutex{}}
}

func nextFileSpec(prefix string, max int) string {
	for i := 0; i < max; i++ {
		spec := fmt.Sprintf("%s.%d", prefix, i+1)
		if !util.FileExists(spec) {
			return spec
		}
	}
	return fmt.Sprintf("%s.%d", prefix, 1)
}

func InitDefaultMonitor(interval time.Duration, maxFileCount int, prefix string) {
	DefaultMonitor = NewMonitor(interval, maxFileCount, prefix)
}
