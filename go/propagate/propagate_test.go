package propagate_test

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sparrowhawktech/toolkit/util"
	"testing"
	"time"
	"toolkit/coverage"
	"toolkit/dispatch"
	"toolkit/propagate"
	"toolkit/propagate/tools"
)

var httpPort = coverage.ResolvePort("propagate.coverage")
var addressPattern = "http://127.0.0.1:%d/node%s"
var nodeMap = make(map[string]*propagate.Node)

const testBroadcastTopic = "testBroadcast"
const testRequestTopic = "testRequest"

func TestMain(m *testing.M) {
	util.ConfigLoggers("propagate.log", 1000000, 2, true, log.Lshortfile|log.Ltime, "info", "error", "propagate.request")
	util.SetDefaultStackTraceTag("error")
	address1 := fmt.Sprintf(addressPattern, httpPort, "1")
	address2 := fmt.Sprintf(addressPattern, httpPort+1, "2")
	address3 := fmt.Sprintf(addressPattern, httpPort+2, "3")
	createNode("node1", []string{}, 0)
	createNode("node2", []string{}, 1)
	createNode("node3", []string{}, 2)
	node4 := createNode("node4", []string{}, 3)
	node4.BootstrapNodes([]string{address1, address2, address3, *node4.Address})
	startNodes()
	m.Run()
}

func startNodes() {
	m := make(map[string]bool)
	max := len(nodeMap)
	ch := make(chan string)
	l := func(topic string, i interface{}) {
		ch <- i.(string)
	}
	dispatch.GlobalDispatcher.RegisterListener(l, propagate.AnnounceSentEventName)
	for _, n := range nodeMap {
		go n.Start()
	}
	for {
		select {
		case a := <-ch:
			util.Log("info").Printf("%s announce received", a)
			m[a] = true
		case <-time.After(time.Second * 3):
			panic("Timeout waiting for nodes announce")
		}
		if len(m) >= max {
			break
		}
	}

	time.Sleep(time.Second * 2)

	for _, v := range nodeMap {
		fmt.Printf("Visible memebr @ %s: \n%s\n", *v.Address, string(util.MarshalPretty(v.Members.VisibleNodesInfo())))
	}

}

func createNode(id string, peers []string, nPort int) *propagate.Node {
	thisPort := httpPort + nPort
	nodeConfig := propagate.NodeConfig{
		Protocol:     util.PStr("http"),
		Host:         util.PStr("127.0.0.1"),
		Port:         &thisPort,
		BasePath:     util.PStr("/" + id),
		PrimaryPeers: peers,
	}
	n := propagate.NewNode(nodeConfig)
	n.RegisterBroadcastHandler(testBroadcastTopic, func(transactionId string, payload []byte, trajectory []string) {
		fmt.Printf("Handler %s received: %s through %v\n", id, string(payload), trajectory)
		dispatch.GlobalDispatcher.Dispatch(testBroadcastTopic, id)
	})
	n.RegisterUnicastHandler(testRequestTopic, func(transactionId string, payload []byte, trajectory []string) []byte {
		fmt.Printf("Handler %s received: %s through %v\n", id, string(payload), trajectory)
		return []byte("Hi mom! This is " + id)
	})

	serveMux := http.NewServeMux()
	n.ConfigureHttpHandlers(serveMux)

	proxyHandler := propagate.NewProxyPacketHandler(serveMux)
	n.RegisterUnicastHandler(propagate.HttpRequestProxyRequestName, proxyHandler.Handle)

	nodeMap[id] = n
	coverage.StartHttpServer(serveMux, thisPort)
	return n
}

func TestBroadcast(t *testing.T) {
	for k, _ := range nodeMap {
		executeBroadcast(k)
	}
}

func executeBroadcast(name string) {
	util.Log("info").Printf("Sending broadcast through %s", name)
	m := make(map[string]bool)
	ch := make(chan string)
	l := func(topic string, i interface{}) {
		ch <- i.(string)
	}
	dispatch.GlobalDispatcher.RegisterListener(l, testBroadcastTopic)
	defer func() {
		dispatch.GlobalDispatcher.UnregisterListener(l, testBroadcastTopic)
	}()
	node := nodeMap[name]
	resultCh := make(chan propagate.BroadcastResults)
	go func() {
		r := node.Broadcast(testBroadcastTopic, []byte(fmt.Sprintf("Hi Mom from #%s!", name)))
		resultCh <- r
	}()
	for {
		select {
		case s := <-ch:
			m[s] = true
		case <-time.After(time.Second * 3):
			panic("Timeout waiting for all nodes to acknowledge")
		}
		if len(m) == len(nodeMap)-1 {
			break
		}
	}
	r := <-resultCh
	g := tools.ComputeBroadcastTransactionGraph(*node.Address, *r.TransactionId, *node.Address)
	checkRedundancy(*g.RootNode, make(map[string]bool))
	util.JsonPretty(g, os.Stdout)

}

func checkRedundancy(node tools.BroadcastTransactionGraphNode, m map[string]bool) {
	redundant, ok := m[*node.Address]
	if ok && redundant && !*node.Redundant {
		panic(fmt.Sprintf("A packet reached destination redundantly and is not marked as such @ %s", *node.Address))
	}
	if !*node.Redundant {
		m[*node.Address] = true
	}
	for _, child := range node.OutboundNodes {
		checkRedundancy(child, m)
	}
}

func TestRequest(t *testing.T) {
	n1 := nodeMap["node1"]
	n4 := nodeMap["node4"]
	for i := 0; i < 10; i++ {
		result := n1.ExecuteRequest(*n4.Address, "testRequest", []byte(fmt.Sprintf("Hi son%d!", i+1)))
		r := tools.ComputeUnicastTransactionGraph(*n1.Address, *result.TransactionId, *n1.Address)
		util.JsonPretty(r, os.Stdout)
	}
}
