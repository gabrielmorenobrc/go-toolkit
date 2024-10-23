package main

import (
	"flag"
	"fmt"
	"os"
	"sparrowhawktech/toolkit/util"
	"time"
	"toolkit/propagate"
	"toolkit/propagate/tools"
	"toolkit/web"
)

var help = util.FlagBool("h", false, "Help")
var broadcast = util.FlagBool("b", false, "Broadcast transaction type")
var unicast = util.FlagBool("u", false, "Unicast transaction type")
var id = util.FlagString("t", "", "Fully qualified transaction id (address.n). If specified trace data will be collected and printed out.")
var peer = util.FlagString("p", "", "Access peer's address for this request")
var rootAddress = util.FlagString("r", "", "Root node's address. Blank means equal to access peer")
var destination = util.FlagString("d", "", "Destination node's address for unicast message")
var topic = util.FlagString("o", "", "Topic for message to be sent")
var payload = util.FlagString("l", "", "Payload data. If specified either a broadcast or unicast message will be sent depending on -b/-u")
var repeat = util.FlagInt("n", 1, "Repeat n times as fast as possible.")
var relayFailureDepth = util.FlagInt("rf", -1, "Forced relay failure depth in the propagation path.")
var processFailureDepth = util.FlagInt("pf", -1, "Forced topic handler failure depth in the propagation path.")
var trace = util.FlagBool("v", false, "If present will issue and print out a trace request right after a request execution.")

type Result struct {
	Data            interface{}   `json:"data"`
	ProcessDuration time.Duration `json:"-"`
	ProcessTime     string        `json:"processTime"`
}

func main() {

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	if *rootAddress == "" {
		rootAddress = util.PStr(*peer)
	}
	if *id != "" {
		runTransactionGraph()
	} else if *broadcast {
		runBroadcast()
	} else if *unicast {
		runUnicast()
	} else {
		flag.Usage()
	}

}

func runTransactionGraph() {
	result := resolveTransactionGraph()
	util.JsonPretty(result, os.Stdout)
}

func resolveTransactionGraph() interface{} {
	if *unicast {
		return tools.ComputeUnicastTransactionGraph(*peer, *id, *rootAddress)
	} else if *broadcast {
		return tools.ComputeBroadcastTransactionGraph(*peer, *id, *rootAddress)
	} else {
		panic("Nothing to run")
	}
}

func runUnicast() {
	hostName, err := os.Hostname()
	util.CheckErr(err)
	address := fmt.Sprintf("%d@%s", os.Getpid(), hostName)
	requestPacket := newRequestPacket(*topic, []byte(*payload), address)
	requestPacket.Recipient = destination
	responsePacket := propagate.UnicastResponsePacket{}

	defer func() {
		if *trace {
			time.Sleep(time.Second)
			traceResult := tools.ComputeUnicastTransactionGraph(*peer, *requestPacket.TransactionId, *peer)
			util.JsonPretty(traceResult, os.Stdout)
		}
	}()
	t0 := time.Now()
	for i := 0; i < *repeat; i++ {
		transactionId := fmt.Sprintf("%s.%d.%d", address, time.Now().Unix(), i)
		requestPacket.TransactionId = &transactionId
		println(transactionId)
		web.PostJson(*peer+"/executeRequest", requestPacket, &responsePacket, 200, time.Second, nil)
	}
	t1 := time.Now()
	d := t1.Sub(t0)
	result := Result{
		Data:            responsePacket,
		ProcessDuration: d,
		ProcessTime:     fmt.Sprintf("%v", d),
	}
	util.JsonPretty(result, os.Stdout)
}

func runBroadcast() {
	hostName, err := os.Hostname()
	util.CheckErr(err)
	address := fmt.Sprintf("%d@%s", os.Getpid(), hostName)
	requestPacket := newRequestPacket(*topic, []byte(*payload), address)
	defer func() {
		if *trace {
			time.Sleep(time.Second)
			traceResult := tools.ComputeBroadcastTransactionGraph(*peer, *requestPacket.TransactionId, *peer)
			util.JsonPretty(traceResult, os.Stdout)
		}
	}()
	responsePacket := propagate.BroadcastResponsePacket{}
	t0 := time.Now()
	for i := 0; i < *repeat; i++ {
		transactionId := fmt.Sprintf("%s.%d.%d", address, time.Now().Unix(), i)
		println(transactionId)
		requestPacket.TransactionId = &transactionId
		web.PostJson(*peer+"/broadcast", requestPacket, &responsePacket, 200, time.Second, nil)
	}
	t1 := time.Now()
	d := t1.Sub(t0)
	result := Result{
		Data:            responsePacket,
		ProcessDuration: d,
		ProcessTime:     fmt.Sprintf("%v", d),
	}
	util.JsonPretty(result, os.Stdout)

}

func newRequestPacket(topic string, payload []byte, sourceAddress string) *propagate.RequestPacket {

	debugInstructions := propagate.DebugInstructions{Trajectory: util.PBool(true)}
	if *relayFailureDepth > -1 {
		debugInstructions.RelayFailureDepth = relayFailureDepth
	}
	if *processFailureDepth > -1 {
		debugInstructions.ProcessFailureDepth = processFailureDepth
	}
	return &propagate.RequestPacket{
		PacketKind:        propagate.UserPacketKind,
		Recipient:         nil,
		Timestamp:         util.PTime(time.Now()),
		Topic:             &topic,
		Payload:           payload,
		Trajectory:        []string{sourceAddress},
		DebugInstructions: &debugInstructions,
	}
}
