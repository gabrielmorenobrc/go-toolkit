package propagate

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sparrowhawktech/toolkit/util"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"toolkit/dispatch"
	"toolkit/web"
)

const (
	AnnounceTopic         = "announce"
	BootstrapTopic        = "bootstrap"
	AnnounceSentEventName = "announceSent"
)

type BootstrapRequest struct {
	Members []string `json:"members"`
}

type InfoResponse struct {
	NodeConfig *NodeConfig   `json:"nodeConfig"`
	Members    []MemberEntry `json:"members"`
}

type BroadcastTransactionStatisticsResponse struct {
	Statistics *BroadcastStatisticsEntry `json:"statistics"`
}

type UnicastTransactionStatisticsResponse struct {
	Statistics *UnicastStatisticsEntry `json:"statistics"`
}

type UnicastRequestHandler func(transactionId string, payload []byte, trajectory []string) []byte
type BroadcastRequestHandler func(transactionId string, payload []byte, trajectory []string)

type NodeConfig struct {
	Protocol     *string
	Host         *string
	Port         *int
	BasePath     *string
	PrimaryPeers []string
}

type Node struct {
	requestHandlerMap   map[string]UnicastRequestHandler
	broadcastHandlerMap map[string]BroadcastRequestHandler
	transactions        *Transactions
	Client              *Client
	ticker              *time.Ticker
	config              *NodeConfig
	Members             *Members
	Address             *string
	Dispatcher          *dispatch.Dispatcher
	lastId              int64
	idMux               *sync.Mutex
	userData            atomic.Pointer[UserData]
	configMux           *sync.Mutex
	broadcastStatistics *BroadcastStatistics
	DebugInstructions   *DebugInstructions
	requestStatistics   *UnicastStatistics
}

func (o *Node) Config() NodeConfig {
	return *o.config
}

func (o *Node) ListenAddress() string {
	return fmt.Sprintf("%s:%d", *o.config.Host, *o.config.Port)
}

func (o *Node) Start() {
	o.Dispatcher.RegisterListener(o.connectionErrorListener, ConnectionErrorEventName)
	o.Announce()
	o.ticker = time.NewTicker(time.Second * 2)
	for range o.ticker.C {
		o.safeAnnounce()
		o.evictNeighbors()
		o.evictTransactions()
		o.evictStatistics()
	}
}

func (o *Node) Stop() {
	if o.ticker != nil {
		o.Dispatcher.UnregisterListener(o.connectionErrorListener, ConnectionErrorEventName)
		o.ticker.Stop()
		o.ticker = nil
	}
}

func (o *Node) evictNeighbors() {
	defer util.CatchPanic()
	o.Members.Evict()
}

func (o *Node) evictTransactions() {
	defer util.CatchPanic()
	o.transactions.Evict(time.Minute)
}

func (o *Node) evictStatistics() {
	defer util.CatchPanic()
	o.broadcastStatistics.Evict()
}

func (o *Node) safeAnnounce() {
	defer util.CatchPanic()
	o.Announce()
}

func (o *Node) nextId() int64 {
	o.idMux.Lock()
	defer o.idMux.Unlock()
	id := o.lastId + 1
	o.lastId = id
	return id
}

func (o *Node) nextUUID() string {
	id := o.nextId()
	uuid := fmt.Sprintf("%s.%d", *o.Address, id) // UUID?
	return uuid
}

func (o *Node) BootstrapNodes(members []string) {
	t0 := time.Now()
	peers := o.extractPendingPeers(members, []string{*o.Address})
	o.ensurePrimaryPeers(peers)
	packet := RequestPacket{
		PacketKind:    SystemPacketKind,
		TransactionId: util.PStr(o.nextUUID()),
		Recipient:     nil,
		Topic:         util.PStr(BootstrapTopic),
		Payload: util.Marshal(BootstrapRequest{
			Members: members,
		}),
		Trajectory:        make([]string, 0),
		DebugInstructions: o.DebugInstructions,
	}
	processingContext := BroadcastProcessingContext{
		RequestPacket: &packet,
		Statistics:    &BroadcastProcessingStatistics{},
	}
	t1 := time.Now()
	d := t1.Sub(t0).Nanoseconds()
	processingContext.Statistics.TotalTime = &d
	go o.doBroadcast(peers, &processingContext)
}

func (o *Node) Announce() {
	t0 := time.Now()
	all := o.AllPeers()
	packet := RequestPacket{
		PacketKind:    SystemPacketKind,
		TransactionId: util.PStr(o.nextUUID()),
		Recipient:     nil,
		Topic:         util.PStr(AnnounceTopic),
		Payload: util.Marshal(AnnounceRequest{
			NodeInfo: &NodeInfo{
				Address:      o.Address,
				PrimaryPeers: o.config.PrimaryPeers,
				UserData:     o.userData.Load(),
			},
			Members: o.Members.VisibleNodesInfo(),
		}),
		Trajectory:        make([]string, 0),
		DebugInstructions: o.DebugInstructions,
	}
	processingContext := BroadcastProcessingContext{
		RequestPacket: &packet,
		Statistics:    &BroadcastProcessingStatistics{},
	}
	t1 := time.Now()
	d := t1.Sub(t0).Nanoseconds()
	processingContext.Statistics.TotalTime = &d
	go o.doBroadcast(all, &processingContext)
	dispatch.GlobalDispatcher.Dispatch(AnnounceSentEventName, *o.Address)
}

/*
*
Handler for unicast request/response flow. Either we are the destination and process the request or we forward
to the next hops we know about until we can complete the blocking flow or fail.
*/
func (o *Node) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	requestPacket := RequestPacket{}
	util.JsonDecode(&requestPacket, r.Body)

	processingContext := &UnicastProcessingContext{
		RequestPacket:   &requestPacket,
		Statistics:      &UnicastProcessingStatistics{},
		ResponseInfoMap: make(map[string]UnicastResponseInfo),
	}

	defer func() {
		if !*processingContext.ResponsePacket.AlreadyProcessed {
			o.requestStatistics.Add(*requestPacket.TransactionId, strings.Join(requestPacket.Trajectory, "/"), *processingContext.Statistics, processingContext.ResponseInfoMap)
		}
	}()

	if *requestPacket.Recipient == *o.Address {
		o.processRequest(processingContext)
	} else {
		o.forwardRequest(processingContext)
	}
	d := time.Since(t0).Nanoseconds()
	processingContext.Statistics.TotalTime = &d
	processingContext.ResponsePacket.ProcessingStatistics = processingContext.Statistics
	processingContext.ResponsePacket.Trajectory = requestPacket.Trajectory

	web.JsonResponse(*processingContext.ResponsePacket, w)

}

func (o *Node) processRequest(processingContext *UnicastProcessingContext) {
	t0 := time.Now()
	o.transactions.AcquireRequestTransaction(*processingContext.RequestPacket.TransactionId)
	defer o.transactions.ReleaseRequestTransaction(*processingContext.RequestPacket.TransactionId, processingContext.ResponsePacket)

	requestPacket := *processingContext.RequestPacket
	responsePacket := UnicastResponsePacket{
		TransactionId:    requestPacket.TransactionId,
		Address:          o.Address,
		AlreadyProcessed: util.PBool(false),
	}
	defer func() {
		if r := recover(); r != nil {
			errorMessage := util.ResolveErrorMessage(r)
			responsePacket.Successful = util.PBool(false)
			responsePacket.Payload = []byte(errorMessage)
			processingContext.Statistics.ProcessingError = &errorMessage
			panic(r)
		}
	}()
	defer func() {
		d := time.Since(t0).Nanoseconds()
		processingContext.Statistics.ProcessTime = &d
		processingContext.ResponsePacket = &responsePacket
	}()
	handler, ok := o.requestHandlerMap[*requestPacket.Topic]
	if !ok {
		panic(fmt.Sprintf("Unsupported topic: %s", *requestPacket.Topic))
	}
	o.forceDebugProcessFailure(requestPacket)
	responsePayload := handler(*requestPacket.TransactionId, requestPacket.Payload, requestPacket.Trajectory)
	responsePacket.Successful = util.PBool(true)
	responsePacket.Payload = responsePayload
}

func (o *Node) forwardRequest(processingContext *UnicastProcessingContext) {
	defer func() {
		if r := recover(); r != nil {
			errorMessage := util.ResolveErrorMessage(r)
			processingContext.Statistics.ForwardingError = &errorMessage
			panic(r)
		}
	}()

	responsePacket, _ := o.resolveRequestTransaction(*processingContext.RequestPacket.TransactionId)
	if responsePacket == nil {

		o.forceDebugRelayFailure(*processingContext.RequestPacket)

		requestPacket := *processingContext.RequestPacket

		remaining := o.resolvePreferredGateways(requestPacket)
		var allAttempted []string
		var lastSentTo []string
		if len(remaining) > 0 {
			remaining = o.extractRemainingAddresses(remaining, requestPacket.Trajectory)
		}

		if len(remaining) > 0 {
			allAttempted = append(allAttempted, remaining...)
			lastSentTo = o.forwardToGateways(remaining, processingContext)
		}

		if processingContext.ResponsePacket == nil {
			remaining = o.extractRemainingAddresses(o.AllPeers(), lastSentTo)
			remaining = o.extractRemainingAddresses(o.AllPeers(), requestPacket.Trajectory)
			if len(remaining) > 0 {
				allAttempted = append(allAttempted, remaining...)
				lastSentTo = o.forwardToGateways(remaining, processingContext)
			}
		}
		if processingContext.ResponsePacket == nil {
			panic(fmt.Sprintf("%s: No more gateways to try for request %s. Tried: %v", *o.Address, *requestPacket.TransactionId, allAttempted))
		}
	} else {
		processingContext.ResponsePacket = responsePacket
	}
}

func (o *Node) resolveRequestTransaction(id string) (*UnicastResponsePacket, bool) {
	responsePacket, isNew := o.transactions.AcquireRequestTransaction(id)
	defer o.transactions.ReleaseRequestTransaction(id, responsePacket)
	return responsePacket, isNew
}

func (o *Node) resolvePreferredGateways(requestPacket RequestPacket) []string {
	memberEntry := o.Members.Find(*requestPacket.Recipient)
	if memberEntry == nil || time.Since(memberEntry.LastTime) > time.Minute {
		return nil
	}
	gatewayAddresses := make([]string, len(memberEntry.Gateways))
	for i, g := range memberEntry.Gateways {
		gatewayAddresses[i] = g.Address
	}
	return gatewayAddresses

}

func (o *Node) forwardToGateways(gateways []string, processingContext *UnicastProcessingContext) []string {
	requestPacket := processingContext.RequestPacket
	util.Log("propagate").Printf("%s: Request %s. Will try remaining nodes: %v", *o.Address, *requestPacket.TransactionId, gateways)
	if len(gateways) == 0 {
		panic(fmt.Sprintf("%s: Attempting to send and 0 remaining nodes to try. TX: %s", *o.Address, *requestPacket.TransactionId))
	}
	ch := make(chan *UnicastResponseInfo)
	for _, a := range gateways {
		go o.requestTo(a, processingContext, ch)
	}
	sentTo := make([]string, 0)
	max := len(gateways)
	n := 0
	select {
	case r := <-ch:
		if r != nil {
			sentTo = append(sentTo, *r.Address)
			processingContext.ResponsePacket = r.ResponsePacket
			return sentTo
		}
		n++
		if n == max {
			break
		}
	}
	return sentTo
}

func (o *Node) requestTo(gatewayAddress string, processingContext *UnicastProcessingContext, ch chan *UnicastResponseInfo) {
	defer util.CatchPanic()
	requestPacket := *processingContext.RequestPacket
	responseInfo := &UnicastResponseInfo{
		Address: &gatewayAddress,
	}
	t0 := time.Now()
	defer func() {
		duration := time.Since(t0)
		nanos := duration.Nanoseconds()
		responseInfo.RTT = &nanos
		processingContext.PutResponseInfo(gatewayAddress, *responseInfo)
	}()
	defer func() {
		if r := recover(); r != nil {
			errorMessage := util.ResolveErrorMessage(r)
			responseInfo.Error = &errorMessage
			ch <- nil
			panic(r)
		}
	}()

	responsePacket := o.doRequestTo(gatewayAddress, requestPacket)
	t1 := time.Now()
	duration := t1.Sub(t0)
	responseInfo.ResponsePacket = &responsePacket
	o.Members.UpdateRTT(*requestPacket.Recipient, gatewayAddress, duration)
	ch <- responseInfo
}

func (o *Node) doRequestTo(gatewayAddress string, requestPacket RequestPacket) (responsePacket UnicastResponsePacket) {
	defer func() {
		if r := recover(); r != nil {
			o.Dispatcher.Dispatch(ConnectionErrorEventName, ConnectionErrorEvent{
				TargetAddress: &gatewayAddress,
				Error:         r,
				RequestPacket: requestPacket,
			})
			panic(r)
		}
	}()
	r := o.Client.requestTo(gatewayAddress, requestPacket, time.Second)
	return r
}

func (o *Node) extractRemainingAddresses(all []string, addresses []string) []string {
	m := make(map[string]bool, len(addresses))
	for _, v := range addresses {
		m[v] = true
	}
	result := make([]string, 0)
	for _, k := range all {
		if _, ok := m[k]; !ok {
			result = append(result, k)
		}
	}
	return result
}

/*
*
Handler for asynchronous messages with no specific destination. We process and send to all our active conenctions, except
for the ones already included the trajetory record.
*/
func (o *Node) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	requestPacket := RequestPacket{}
	util.JsonDecode(&requestPacket, r.Body)
	found := o.transactions.PutBroadcastTransaction(*requestPacket.TransactionId)
	if requestPacket.PacketKind == UserPacketKind {
		util.Log("propagate").Printf("%s: Received broadcast TX %s through %v", *o.Address, *requestPacket.TransactionId, requestPacket.Trajectory)
	}
	responsePacket := BroadcastResponsePacket{
		TransactionId:    requestPacket.TransactionId,
		Address:          o.Address,
		AlreadyProcessed: &found,
	}
	if requestPacket.DebugInstructions != nil && *requestPacket.DebugInstructions.Trajectory {
		responsePacket.Trajectory = requestPacket.Trajectory
	}
	if !found {
		go o.processBroadcastRequest(requestPacket)
	} else if requestPacket.PacketKind == UserPacketKind {
		util.Log("propagate").Printf("%s: Request %s already processed", *o.Address, *requestPacket.TransactionId)
	}
	web.JsonResponse(responsePacket, w)
}

func (o *Node) processBroadcastRequest(requestPacket RequestPacket) {
	defer util.CatchPanic()
	t0 := time.Now()
	processingContext := BroadcastProcessingContext{
		RequestPacket: &requestPacket,
		Statistics:    &BroadcastProcessingStatistics{},
	}
	defer func() {
		o.broadcastStatistics.PutProcessingStatistics(*requestPacket.TransactionId, *processingContext.Statistics)
	}()
	defer func() {
		if r := recover(); r != nil {
			errorMessage := util.ResolveErrorMessage(r)
			processingContext.Statistics.BroadcastError = &errorMessage
			panic(r)
		}
	}()
	if requestPacket.PacketKind == SystemPacketKind {
		o.processSystemBroadcast(&processingContext)
	} else if requestPacket.PacketKind == UserPacketKind {
		o.processUserBroadcast(&processingContext)
	} else {
		panic(fmt.Sprintf("Unsupported PacketKind: %v", requestPacket.PacketKind))
	}
	t1 := time.Now()
	d := t1.Sub(t0).Nanoseconds()
	processingContext.Statistics.ProcessTime = &d

	pendingPeers := o.extractPendingPeers(o.AllPeers(), requestPacket.Trajectory)
	util.Log("propagate").Printf("%s: Pending peers for request %s: %v", *o.Address, *processingContext.RequestPacket.TransactionId, pendingPeers)

	o.forceDebugRelayFailure(requestPacket)
	if len(pendingPeers) > 0 {
		o.doBroadcast(pendingPeers, &processingContext)
	}
	d2 := time.Since(t0).Nanoseconds()
	processingContext.Statistics.TotalTime = &d2
}

func (o *Node) processSystemBroadcast(processingContext *BroadcastProcessingContext) {
	requestPacket := processingContext.RequestPacket
	if *requestPacket.Topic == AnnounceTopic {
		announceRequest := AnnounceRequest{}
		util.Unmarshal(requestPacket.Payload, &announceRequest)
		if *announceRequest.NodeInfo.Address != *o.Address {
			o.Members.Put(*announceRequest.NodeInfo.Address, *announceRequest.NodeInfo)
		}
	} else if *requestPacket.Topic == BootstrapTopic {
		bootstrapRequest := BootstrapRequest{}
		util.Unmarshal(requestPacket.Payload, &bootstrapRequest)
		o.ensurePrimaryPeers(o.extractPendingPeers(bootstrapRequest.Members, []string{*o.Address}))
	} else {
		panic(fmt.Sprintf("Unsupported system topic: %s", *requestPacket.Topic))
	}
}

func (o *Node) processUserBroadcast(processingContext *BroadcastProcessingContext) {
	defer util.CatchPanic()
	defer func() {
		if r := recover(); r != nil {
			message := util.ResolveErrorMessage(r)
			processingContext.Statistics.TopicHandlerError = &message
			panic(r)
		}
	}()
	packet := *processingContext.RequestPacket
	o.forceDebugProcessFailure(packet)
	if handler, ok := o.broadcastHandlerMap[*packet.Topic]; ok {
		handler(*packet.TransactionId, packet.Payload, packet.Trajectory)
	}
}

func (o *Node) extractPendingPeers(peers []string, visited []string) []string {
	mVisited := make(map[string]bool, len(visited))
	for _, v := range visited {
		mVisited[v] = true
	}
	mVisited[*o.Address] = true
	result := make([]string, 0)
	for _, v := range peers {
		if _, ok := mVisited[v]; !ok {
			result = append(result, v)
		}
	}
	return result
}

func (o *Node) RegisterUnicastHandler(topic string, handler UnicastRequestHandler) {
	o.requestHandlerMap[topic] = handler
}

func (o *Node) RegisterBroadcastHandler(topic string, handler BroadcastRequestHandler) {
	o.broadcastHandlerMap[topic] = handler
}

func (o *Node) ExecuteRequest(recipient string, topic string, payload []byte) ExecuteRequestResults {
	requestPacket := RequestPacket{
		TransactionId:     util.PStr(o.nextUUID()),
		Recipient:         &recipient,
		Timestamp:         util.PTime(time.Now()),
		Topic:             &topic,
		Payload:           payload,
		DebugInstructions: o.DebugInstructions,
		Trajectory:        nil,
	}
	processingContext := UnicastProcessingContext{
		RequestPacket:   &requestPacket,
		Statistics:      &UnicastProcessingStatistics{},
		ResponseInfoMap: make(map[string]UnicastResponseInfo),
	}
	o.forwardRequest(&processingContext)
	o.requestStatistics.Add(*requestPacket.TransactionId, *o.Address, *processingContext.Statistics, processingContext.ResponseInfoMap)
	responsePacket := processingContext.ResponsePacket
	if !*responsePacket.Successful {
		panic(responsePacket)
	}
	return ExecuteRequestResults{
		Payload:            responsePacket.Payload,
		Statistics:         processingContext.Statistics,
		ResponseStatistics: processingContext.ResponseInfoMap,
		TransactionId:      requestPacket.TransactionId,
	}
}

func (o *Node) Broadcast(topic string, payload []byte) BroadcastResults {
	t0 := time.Now()
	peers := o.AllPeers()
	util.Log("propagate").Printf("All peers @ %s: %v", *o.Address, peers)
	requestPacket := &RequestPacket{
		TransactionId:     util.PStr(o.nextUUID()),
		PacketKind:        UserPacketKind,
		Topic:             &topic,
		Timestamp:         util.PTime(time.Now()),
		Payload:           payload,
		Trajectory:        make([]string, 0),
		DebugInstructions: o.DebugInstructions,
	}
	processingContext := BroadcastProcessingContext{
		RequestPacket: requestPacket,
		Statistics:    &BroadcastProcessingStatistics{},
	}
	o.doBroadcast(peers, &processingContext)
	t1 := time.Now()
	d := t1.Sub(t0).Nanoseconds()
	processingContext.Statistics.TotalTime = &d
	return BroadcastResults{
		TransactionId:      requestPacket.TransactionId,
		Statistics:         processingContext.Statistics,
		GatewaysStatistics: processingContext.GatewaysStatistics,
	}
}

func (o *Node) doBroadcast(peers []string, processingContext *BroadcastProcessingContext) {
	defer util.CatchPanic()
	gatewayAddressesMap := o.resolveGatewayAddressesForPeers(peers)
	util.Log("propagate").Printf("gatewayAddressesMap @ %s: %v", *o.Address, gatewayAddressesMap)
	if len(gatewayAddressesMap) > 0 {
		responses := o.broadcastToPeers(gatewayAddressesMap, *processingContext.RequestPacket)
		processingContext.GatewaysStatistics = responses
		o.broadcastStatistics.PutResponses(*processingContext.RequestPacket.TransactionId, responses)
	}
}

func (o *Node) resolveGatewayAddressesForPeers(peers []string) map[string][]string {
	result := make(map[string][]string)
	for _, peer := range peers {
		result[peer] = make([]string, 0)
	}
	for _, peer := range peers {
		gateways := o.preferredGatewayAddressesForPeer(peer)
		result[peer] = gateways
	}
	return result
}

func (o *Node) preferredGatewayAddressesForPeer(recipient string) []string {
	memberEntry := o.Members.Find(recipient)
	if memberEntry != nil && len(memberEntry.Gateways) > 0 {
		result := make([]string, 0)
		for _, e := range memberEntry.Gateways { // These are already sorted by last response time
			gateway := o.Members.Find(e.Address)
			if gateway != nil && gateway.Visible {
				result = append(result, e.Address)
			}
		}
		return result
	} else {
		return o.AllPeers()
	}
}

func (o *Node) broadcastToPeers(bestPaths map[string][]string, requestPacket RequestPacket) map[string]BroadcastResponseInfo {
	responsesCh := make(chan *BroadcastResponseInfo)
	responseMap := make(map[string]BroadcastResponseInfo)
	go o.collectResponses(responsesCh, responseMap)
	o.fanout(bestPaths, requestPacket, responsesCh)
	return responseMap
}

func (o *Node) fanout(bestPaths map[string][]string, requestPacket RequestPacket, ch chan *BroadcastResponseInfo) {
	defer func() { ch <- nil }()
	wg := new(sync.WaitGroup)
	n := 0
	peers := make([]string, 0)
	for peer, _ := range bestPaths {
		if peer != *o.Address {
			peers = append(peers, peer)
			n++
		}
	}
	if n > 0 {
		wg.Add(n)
		for _, peer := range peers {
			go o.broadcastTo(peer, requestPacket, wg, ch)
		}
		util.Log("propagate").Print("BEGIN WAIT")
		wg.Wait()
		util.Log("propagate").Print("END WAIT")
	}
}

func (o *Node) collectResponses(ch chan *BroadcastResponseInfo, responseMap map[string]BroadcastResponseInfo) {
	defer util.CatchPanic()
	for i := range ch {
		if i == nil {
			util.Log("propagate").Print("Exit collectResponses(nil)")
			break
		}
		responseMap[*i.GatewayAddress] = *i
	}
}

func (o *Node) broadcastTo(gatewayAddress string, packet RequestPacket, wg *sync.WaitGroup, ch chan *BroadcastResponseInfo) {
	t0 := time.Now()
	defer util.CatchPanic()
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			d := time.Since(t0).Nanoseconds()
			message := util.ResolveErrorMessage(r)
			util.Log("propagate").Printf(" <- %s", message)

			ch <- &BroadcastResponseInfo{
				GatewayAddress: &gatewayAddress,
				TransactionId:  packet.TransactionId,
				ResponsePacket: nil,
				RTT:            &d,
				Error:          &message,
			}
			util.Log("propagate").Printf(" <- %s done", message)

		}
	}()
	r := o.Client.sendBroadcastTo(gatewayAddress, packet, BrentsUniversalKConstant)
	d := time.Since(t0).Nanoseconds()
	ch <- &BroadcastResponseInfo{
		TransactionId:  packet.TransactionId,
		GatewayAddress: &gatewayAddress,
		ResponsePacket: &r,
		RTT:            &d,
	}
}

func (o *Node) AllPeers() []string {
	result := make([]string, 0)
	n := o.Members.VisibleNodesInfo()
	for k, _ := range n {
		result = append(result, k)
	}
	for _, v := range o.config.PrimaryPeers {
		if _, ok := n[v]; !ok {
			result = append(result, v)
		}
	}
	return result
}

func (o *Node) connectionErrorListener(topic string, i interface{}) {
	event := i.(ConnectionErrorEvent) // assess whether actual networking error to mark not visible
	o.Members.MarkNotVisible(*event.TargetAddress)
	/*	util.Log("debug").Printf("%s marked not visible due to error: %v. Request:\n%s\n",
	 *event.TargetAddress, event.Error, string(util.Marshal(event.RequestPacket)))*/
}

func (o *Node) showAllPeers(w http.ResponseWriter, r *http.Request) {
	result := o.AllPeers()
	web.JsonResponse(result, w)
}

func (o *Node) info(w http.ResponseWriter, r *http.Request) {
	result := InfoResponse{
		NodeConfig: o.config,
		Members:    o.Members.Snapshot(),
	}
	web.JsonResponsePretty(result, w)
}

func (o *Node) broadcastTransactionStatistics(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("transactionId")
	if id == "" {
		panic("Empty id")
	}
	s := o.broadcastStatistics.Find(id)
	web.JsonResponsePretty(BroadcastTransactionStatisticsResponse{Statistics: s}, w)
}

func (o *Node) unicastTransactionStatistics(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("transactionId")
	if id == "" {
		panic("Empty id")
	}
	s := o.requestStatistics.Find(id)
	web.JsonResponsePretty(UnicastTransactionStatisticsResponse{Statistics: s}, w)
}

func (o *Node) HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	targetAddress := r.Header.Get(ProxyTargetAddressHeaderName)
	postData, err := io.ReadAll(r.Body)
	util.CheckErr(err)
	prefix := *o.config.BasePath + "/proxy"
	path := r.URL.Path[len(prefix):]
	proxyData := ProxyRequestData{
		Path:     &path,
		RawQuery: &r.URL.RawQuery,
		Header:   r.Header,
		PostData: postData,
	}
	result := o.ExecuteProxyRequest(proxyData, targetAddress)
	w.WriteHeader(*result.StatusCode)
	util.Write(w, result.Data)
}

func (o *Node) ExecuteProxyRequest(proxyData ProxyRequestData, targetAddress string) ProxyRequestResult {
	payload := util.Marshal(proxyData)
	executeRequestResult := o.ExecuteRequest(targetAddress, HttpRequestProxyRequestName, payload)
	result := ProxyRequestResult{}
	util.Unmarshal(executeRequestResult.Payload, &result)
	return result
}

func (o *Node) forceDebugRelayFailure(packet RequestPacket) {
	if packet.DebugInstructions != nil && packet.DebugInstructions.RelayFailureDepth != nil {
		if len(packet.Trajectory)+1 == *packet.DebugInstructions.RelayFailureDepth {
			panic("Forced debug relay failure")
		}
	}
}

func (o *Node) forceDebugProcessFailure(packet RequestPacket) {
	if packet.DebugInstructions != nil && packet.DebugInstructions.ProcessFailureDepth != nil {
		if len(packet.Trajectory)+1 == *packet.DebugInstructions.ProcessFailureDepth {
			panic("Forced debug topic handler failure")
		}
	}
}

func (o *Node) ConfigureHttpHandlers(mux *http.ServeMux) {
	basePath := "/propagate"
	if o.config.BasePath != nil {
		basePath = *o.config.BasePath
	}
	web.HandleDefault(mux, basePath+ExecuteRequestResourceName, o.handleExecuteRequest)
	web.HandleDefault(mux, basePath+BroadcastResourceName, o.handleBroadcast)
	web.HandleDefault(mux, basePath+"/info", o.info)
	web.HandleDefault(mux, basePath+"/allPeers", o.showAllPeers)
	web.HandleDefault(mux, basePath+"/broadcastTransactionStatistics", o.broadcastTransactionStatistics)
	web.HandleDefault(mux, basePath+"/unicastTransactionStatistics", o.unicastTransactionStatistics)
	web.HandleDefault(mux, basePath+"/proxy/", o.HandleProxyRequest)
	web.HandleDefault(mux, basePath+"/bootstrap", o.handleBootstrap)
	proxyPacketHandler := NewProxyPacketHandler(mux)
	o.RegisterUnicastHandler(HttpRequestProxyRequestName, proxyPacketHandler.Handle)
}

func (o *Node) UpdateUserData(data *UserData) {
	o.userData.Store(data)
}

func (o *Node) UpdateConfig(config NodeConfig) {
	o.Members.ClearAll()
	o.config = &config
}

func (o *Node) UpdatePrimaryPeers(members []string) {
	peers := o.extractPendingPeers(members, []string{*o.Address})
	o.ensurePrimaryPeers(peers)
}

func (o *Node) ensurePrimaryPeers(peers []string) {
	o.configMux.Lock()
	defer o.configMux.Unlock()
	current := o.config.PrimaryPeers
	m := make(map[string]bool)
	for _, p := range current {
		m[p] = true
	}
	for _, p := range peers {
		if _, ok := m[p]; !ok {
			current = append(current, p)
		}
	}
	o.config.PrimaryPeers = current
}

func (o *Node) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	members := make([]string, 0)
	web.ParseParamOrBody(r, &members)
	o.BootstrapNodes(members)
}

func NewNode(config NodeConfig) *Node {
	address := fmt.Sprintf("%s://%s:%d%s", *config.Protocol, *config.Host, *config.Port, *config.BasePath)
	dispatcher := dispatch.NewDispatcher()
	if *config.Host == "" {
		panic("Host can't be empty")
	}
	_, err := url.Parse(address)
	util.CheckErr(err)
	node := &Node{
		requestHandlerMap:   make(map[string]UnicastRequestHandler),
		broadcastHandlerMap: make(map[string]BroadcastRequestHandler),
		transactions:        NewTransactions(),
		broadcastStatistics: NewBroadcastStatistics(),
		requestStatistics:   NewUnicastStatistics(),
		Client:              NewClient(address, dispatcher),
		config:              &config,
		Members:             NewMembers(time.Second*10, dispatcher),
		Address:             &address,
		Dispatcher:          dispatcher,
		lastId:              0,
		idMux:               &sync.Mutex{},
		configMux:           &sync.Mutex{},
	}

	return node
}
