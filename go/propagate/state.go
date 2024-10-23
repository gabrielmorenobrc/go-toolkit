package propagate

import (
	"sort"
	"sparrowhawktech/toolkit/util"
	"sync"
	"time"

	"toolkit/dispatch"
)

const MembersChangedEventName = "propagation.membersChanged"

type GatewayInfo struct {
	Address    string         `json:"address"`
	UpdateTime *time.Time     `json:"time"`
	RTT        *time.Duration `json:"rtt"`
}

type MemberEntry struct {
	NodeInfo NodeInfo
	LastTime time.Time
	Visible  bool
	Gateways []GatewayInfo
}

type Members struct {
	mux               *sync.Mutex
	entries           map[string]MemberEntry
	evictionTolerance *time.Duration
	dispatcher        *dispatch.Dispatcher
}

func (o *Members) Put(address string, info NodeInfo) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry, ok := o.entries[address]
	if !ok {
		entry = MemberEntry{
			Gateways: make([]GatewayInfo, 0),
		}
		for _, e := range o.entries {
			nullDuration := time.Second * 10
			if e.NodeInfo.Address != nil {
				o.updateRTTUnsafe(*e.NodeInfo.Address, address, nullDuration)
				entry.Gateways = append(entry.Gateways, GatewayInfo{
					Address:    *e.NodeInfo.Address,
					UpdateTime: util.PTime(time.Now()),
					RTT:        &nullDuration,
				})
			}
		}
	}
	entry.LastTime = time.Now()
	entry.NodeInfo = info
	entry.Visible = true
	o.entries[address] = entry
	o.dispatcher.Dispatch(MembersChangedEventName, nil)
}

func (o *Members) Find(address string) *MemberEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	if entry, ok := o.entries[address]; ok {
		return &entry
	} else {
		return nil
	}
}

func (o *Members) Snapshot() []MemberEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	result := make([]MemberEntry, len(o.entries))
	i := 0
	for _, e := range o.entries {
		result[i] = e
		i++
	}
	return result
}

func (o *Members) Addresses() []string {
	o.mux.Lock()
	defer o.mux.Unlock()
	result := make([]string, 0)
	now := time.Now()
	for _, e := range o.entries {
		if now.Sub(e.LastTime) < *o.evictionTolerance && e.Visible {
			result = append(result, *e.NodeInfo.Address)
		}
	}
	return result
}

func (o *Members) VisibleNodesInfo() map[string]NodeInfo {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.unsafeVisibleSnapshot()
}

func (o *Members) unsafeVisibleSnapshot() map[string]NodeInfo {
	result := make(map[string]NodeInfo, len(o.entries))
	now := time.Now()
	for k, v := range o.entries {
		if now.Sub(v.LastTime) < *o.evictionTolerance {
			result[k] = v.NodeInfo
		}
	}
	return result
}

func (o *Members) deleteAddressFromPath(address string) {
	for idx, e := range o.entries {
		for idx, gateway := range e.Gateways {
			if gateway.Address == address {
				e.Gateways = append(e.Gateways[:idx], e.Gateways[idx+1:]...)
				break
			}
		}
		o.entries[idx] = e
	}
}

func (o *Members) Evict() {
	o.mux.Lock()
	defer o.mux.Unlock()
	now := time.Now()
	changed := false
	for k, e := range o.entries {
		if now.Sub(e.LastTime) >= *o.evictionTolerance {
			delete(o.entries, k)
			o.deleteAddressFromPath(k)
			changed = true
		}
	}
	if changed {
		o.dispatcher.Dispatch(MembersChangedEventName, nil)
	}
}

func (o *Members) ClearAll() {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.entries = make(map[string]MemberEntry, 0)
}

func (o *Members) MarkNotVisible(address string) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry := o.entries[address]
	entry.Visible = false
	o.entries[address] = entry
}

func (o *Members) UpdateRTT(recipient string, gatewayAddress string, d time.Duration) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.updateRTTUnsafe(recipient, gatewayAddress, d)
}

func (o *Members) updateRTTUnsafe(recipient string, gatewayAddress string, d time.Duration) {
	entry := o.entries[recipient]
	found := false
	for i, g := range entry.Gateways {
		if g.Address == gatewayAddress {
			g.UpdateTime = util.PTime(time.Now())
			g.RTT = &d
			entry.Gateways[i] = g
			found = true
			break
		}
	}
	if !found {
		entry.Gateways = append(entry.Gateways, GatewayInfo{
			Address:    gatewayAddress,
			UpdateTime: util.PTime(time.Now()),
			RTT:        &d,
		})
	}
	sort.Slice(entry.Gateways, func(i, j int) bool {
		return *entry.Gateways[i].RTT < *entry.Gateways[j].RTT
	})
	o.entries[recipient] = entry
}

func NewMembers(evictionTolerance time.Duration, dispatcher *dispatch.Dispatcher) *Members {
	return &Members{
		mux:               &sync.Mutex{},
		entries:           make(map[string]MemberEntry),
		evictionTolerance: &evictionTolerance,
		dispatcher:        dispatcher,
	}
}

type transactionEntry struct {
	Id                    string
	Time                  time.Time
	UnicastResponsePacket *UnicastResponsePacket
	mux                   *sync.Mutex
}

type Transactions struct {
	mux          *sync.Mutex
	transactions map[string]*transactionEntry
}

func (o *Transactions) FindRequestTransaction(id string) *UnicastResponsePacket {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry, ok := o.transactions[id]
	if ok {
		return entry.UnicastResponsePacket
	} else {
		return nil
	}
}

func (o *Transactions) AcquireRequestTransaction(id string) (*UnicastResponsePacket, bool) {
	entry, isNew := o.resolveEntry(id)
	entry.mux.Lock()
	return entry.UnicastResponsePacket, isNew
}

func (o *Transactions) resolveEntry(id string) (*transactionEntry, bool) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry, ok := o.transactions[id]
	if !ok {
		entry = &transactionEntry{
			Id:   id,
			Time: time.Now(),
			mux:  &sync.Mutex{},
		}
		o.transactions[id] = entry
	}
	return entry, !ok
}
func (o *Transactions) ReleaseRequestTransaction(id string, packet *UnicastResponsePacket) {
	entry := o.retrieveEntry(id)
	defer entry.mux.Unlock()
	entry.UnicastResponsePacket = packet
}

func (o *Transactions) retrieveEntry(id string) *transactionEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry, ok := o.transactions[id]
	if !ok {
		panic("Unknown transaction id")
	}
	return entry
}

func (o *Transactions) PutBroadcastTransaction(id string) bool {
	o.mux.Lock()
	defer o.mux.Unlock()
	if _, ok := o.transactions[id]; ok {
		return true
	} else {
		entry := transactionEntry{
			Id:   id,
			Time: time.Now(),
		}
		o.transactions[id] = &entry
		return false
	}
}

func (o *Transactions) snapshot() map[string]*transactionEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	result := make(map[string]*transactionEntry, len(o.transactions))
	for k, v := range o.transactions {
		result[k] = v
	}
	return result
}

func (o *Transactions) Evict(timeToLive time.Duration) {
	obsolete := make([]string, 0)
	snapshot := o.snapshot()
	baseLine := time.Now().Add(timeToLive * -1)
	for _, v := range snapshot {
		if v.Time.Before(baseLine) {
			obsolete = append(obsolete, v.Id)
		}
	}
	o.mux.Lock()
	defer o.mux.Unlock()
	for _, id := range obsolete {
		delete(o.transactions, id)
	}
}

func NewTransactions() *Transactions {
	return &Transactions{
		mux:          &sync.Mutex{},
		transactions: make(map[string]*transactionEntry),
	}
}

type BroadcastStatisticsEntry struct {
	UpdateTime           *time.Time
	TransactionId        *string
	Responses            map[string]BroadcastResponseInfo
	ProcessingStatistics *BroadcastProcessingStatistics
}

type BroadcastStatistics struct {
	mux     *sync.Mutex
	entries map[string]BroadcastStatisticsEntry
}

func (o *BroadcastStatistics) PutProcessingStatistics(transactionId string, statistics BroadcastProcessingStatistics) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry := o.resolveEntry(transactionId)
	entry.ProcessingStatistics = &statistics
	now := time.Now()
	entry.UpdateTime = &now
	o.entries[transactionId] = entry
}

func (o *BroadcastStatistics) PutResponses(transactionId string, responses map[string]BroadcastResponseInfo) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry := o.resolveEntry(transactionId)
	entry.Responses = responses
	now := time.Now()
	entry.UpdateTime = &now
	o.entries[transactionId] = entry
}

func (o *BroadcastStatistics) resolveEntry(id string) BroadcastStatisticsEntry {
	e, ok := o.entries[id]
	if !ok {
		e = BroadcastStatisticsEntry{TransactionId: &id}
		o.entries[id] = e
	}
	return e
}

func (o *BroadcastStatistics) Find(transactionId string) *BroadcastStatisticsEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	if e, ok := o.entries[transactionId]; ok {
		return &e
	} else {
		return nil
	}
}

func (o *BroadcastStatistics) Evict() {
	o.mux.Lock()
	defer o.mux.Unlock()
	obsolete := make([]string, 0, len(o.entries))
	now := time.Now()
	for k, v := range o.entries {
		if now.Sub(*v.UpdateTime) > time.Minute*5 {
			obsolete = append(obsolete, k)
		}
	}
	for _, k := range obsolete {
		delete(o.entries, k)
	}
}

func NewBroadcastStatistics() *BroadcastStatistics {
	return &BroadcastStatistics{
		mux:     &sync.Mutex{},
		entries: make(map[string]BroadcastStatisticsEntry),
	}
}

type UnicastRequestsProcessingStatistics struct {
	Responses            map[string]UnicastResponseInfo `json:"responses"`
	ProcessingStatistics *UnicastProcessingStatistics   `json:"processingStatistics"`
}

type UnicastStatisticsEntry struct {
	UpdateTime    *time.Time                                     `json:"updateTime"`
	TransactionId *string                                        `json:"transaction_id"`
	Requests      map[string]UnicastRequestsProcessingStatistics `json:"requests"`
}

type UnicastStatistics struct {
	mux     *sync.Mutex
	entries map[string]UnicastStatisticsEntry
}

func (o *UnicastStatistics) Add(transactionId string, requestKey string, statistics UnicastProcessingStatistics,
	responses map[string]UnicastResponseInfo) {
	o.mux.Lock()
	defer o.mux.Unlock()
	entry := o.resolveEntry(transactionId)
	request, ok := entry.Requests[requestKey]
	if !ok {
		request = UnicastRequestsProcessingStatistics{
			Responses:            responses,
			ProcessingStatistics: &statistics,
		}
		entry.Requests[requestKey] = request
	}
	now := time.Now()
	entry.UpdateTime = &now
	o.entries[transactionId] = entry
}

func (o *UnicastStatistics) resolveEntry(id string) UnicastStatisticsEntry {
	e, ok := o.entries[id]
	if !ok {
		e = UnicastStatisticsEntry{TransactionId: &id, Requests: make(map[string]UnicastRequestsProcessingStatistics)}
		o.entries[id] = e
	}
	return e
}

func (o *UnicastStatistics) Find(transactionId string) *UnicastStatisticsEntry {
	o.mux.Lock()
	defer o.mux.Unlock()
	if e, ok := o.entries[transactionId]; ok {
		return &e
	} else {
		return nil
	}
}

func (o *UnicastStatistics) Evict() {
	o.mux.Lock()
	defer o.mux.Unlock()
	obsolete := make([]string, 0, len(o.entries))
	now := time.Now()
	for k, v := range o.entries {
		if now.Sub(*v.UpdateTime) > time.Minute*5 {
			obsolete = append(obsolete, k)
		}
	}
	for _, k := range obsolete {
		delete(o.entries, k)
	}
}

func NewUnicastStatistics() *UnicastStatistics {
	return &UnicastStatistics{
		mux:     &sync.Mutex{},
		entries: make(map[string]UnicastStatisticsEntry),
	}
}
