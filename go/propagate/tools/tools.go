package tools

import (
	"fmt"
	"sparrowhawktech/toolkit/util"
	"strings"
	"time"
	"toolkit/propagate"
	"toolkit/web"
)

type BroadcastTransactionGraphNode struct {
	Address             *string                                  `json:"address"`
	RTT                 *int64                                   `json:"rtt"`
	RTTLegend           *string                                  `json:"rttLegend"`
	Redundant           *bool                                    `json:"redundant"`
	BroadcastStatistics *propagate.BroadcastProcessingStatistics `json:"broadcastStatistics"`
	Trajectory          []string                                 `json:"trajectory"`
	OutboundNodes       []BroadcastTransactionGraphNode          `json:"outboundNodes"`
	Error               *string                                  `json:"error"`
}

type BroadcastTransactionGraph struct {
	TransactionId *string                        `json:"transactionId"`
	RootNode      *BroadcastTransactionGraphNode `json:"rootNode"`
}

func ComputeBroadcastTransactionGraph(peer string, transactionId string, rootAddress string) BroadcastTransactionGraph {
	result := BroadcastTransactionGraph{
		TransactionId: &transactionId,
		RootNode: &BroadcastTransactionGraphNode{
			Address:             &rootAddress,
			RTT:                 nil,
			BroadcastStatistics: nil,
			OutboundNodes:       nil,
			Redundant:           util.PBool(false),
		},
	}

	m := make(map[string]bool)
	loadBroadcastTransactionNode(peer, transactionId, result.RootNode, m)
	return result
}

func loadBroadcastTransactionNode(peer string, transactionId string, node *BroadcastTransactionGraphNode, m map[string]bool) {
	r := requestBroadcastNodeStatistics(peer, transactionId, *node.Address)
	if r != nil && r.Statistics != nil {
		node.BroadcastStatistics = r.Statistics.ProcessingStatistics
		for _, v := range r.Statistics.Responses {
			cyclicKey := fmt.Sprintf("%s.%s", *node.Address, *v.GatewayAddress)
			if _, ok := m[cyclicKey]; ok {
				continue
			}
			rttLegend := fmt.Sprintf("%v", time.Nanosecond*time.Duration(*v.RTT))
			child := &BroadcastTransactionGraphNode{
				Address:    v.GatewayAddress,
				RTT:        v.RTT,
				RTTLegend:  &rttLegend,
				Trajectory: append(node.Trajectory, *node.Address),
				Error:      v.Error,
			}
			m[cyclicKey] = true
			if v.ResponsePacket != nil {
				child.Redundant = v.ResponsePacket.AlreadyProcessed
				if !*v.ResponsePacket.AlreadyProcessed {
					loadBroadcastTransactionNode(peer, transactionId, child, m)
				}
			}
			node.OutboundNodes = append(node.OutboundNodes, *child)
		}
	}
}

func requestBroadcastNodeStatistics(peerAddress string, transactionId string, nodeAddress string) *propagate.BroadcastTransactionStatisticsResponse {
	defer util.CatchPanic()
	r := propagate.BroadcastTransactionStatisticsResponse{}
	if peerAddress == nodeAddress {
		web.GetJson(nodeAddress+"/broadcastTransactionStatistics?transactionId="+transactionId, time.Second, 200,
			nil, &r)
	} else {
		name := nodeAddress[strings.LastIndex(nodeAddress, "/"):]
		path := fmt.Sprintf("/proxy/%s/broadcastTransactionStatistics", name)
		web.GetJson(peerAddress+path+"?transactionId="+transactionId, time.Second, 200,
			map[string]string{propagate.ProxyTargetAddressHeaderName: nodeAddress}, &r)
	}
	return &r
}

type UnicastResponses struct {
	RequestKey         *string                                `json:"requestKey"`
	ResponseStatistics *propagate.UnicastProcessingStatistics `json:"responseStatistics"`
	Trajectory         []string                               `json:"trajectory"`
	OutboundNodes      []UnicastTransactionGraphNode          `json:"outboundNodes"`
}

type UnicastTransactionGraphNode struct {
	Key                  *string                                `json:"key"`
	Address              *string                                `json:"address"`
	ProcessingStatistics *propagate.UnicastProcessingStatistics `json:"processingStatistics"`
	RTT                  *int64                                 `json:"rtt"`
	RTTLegend            *string                                `json:"rttLegend"`
	Redundant            *bool                                  `json:"redundant"`
	ErrorResponse        *string                                `json:"errorResponse"`
	Responses            *UnicastResponses                      `json:"responses"`
}

type UnicastTransactionGraph struct {
	TransactionId *string                      `json:"transactionId"`
	RootNode      *UnicastTransactionGraphNode `json:"rootNode"`
}

func ComputeUnicastTransactionGraph(peerAddress string, transactionId string, rootAddress string) UnicastTransactionGraph {
	result := UnicastTransactionGraph{
		TransactionId: &transactionId,
		RootNode: &UnicastTransactionGraphNode{
			Address:   &rootAddress,
			Redundant: util.PBool(false),
			Responses: nil,
		},
	}

	cyclicKeys := make(map[string]bool)
	cache := make(map[string]*propagate.UnicastStatisticsEntry)
	loadUnicastTransactionNode(peerAddress, transactionId, "", result.RootNode, cache, cyclicKeys)
	return result
}

func loadUnicastTransactionNode(peerAddress, transactionId string, prefix string, node *UnicastTransactionGraphNode, cache map[string]*propagate.UnicastStatisticsEntry, m map[string]bool) {
	statisticsEntry, ok := cache[*node.Address]
	if !ok {
		statisticsEntry = requestUnicastNodeStatistics(peerAddress, transactionId, *node.Address)
		cache[*node.Address] = statisticsEntry
	}

	if statisticsEntry != nil {
		for requestKey, requestStatistics := range statisticsEntry.Requests {
			if !strings.HasPrefix(requestKey, prefix) {
				continue
			}
			key := *node.Address
			if len(prefix) > 0 {
				key = prefix + "/" + key
			}
			loadUnicastResponses(peerAddress, transactionId, key, node, requestStatistics, cache, m)
		}
	}
}

func loadUnicastResponses(peerAddress string, transactionId string, prefix string, node *UnicastTransactionGraphNode, requestStatistics propagate.UnicastRequestsProcessingStatistics, cache map[string]*propagate.UnicastStatisticsEntry, m map[string]bool) {
	data := UnicastResponses{RequestKey: &prefix}
	node.ProcessingStatistics = requestStatistics.ProcessingStatistics
	for _, responseInfo := range requestStatistics.Responses {
		key := *responseInfo.Address
		if len(prefix) > 0 {
			key = prefix + "/" + key
		}
		rttLegend := fmt.Sprintf("%v", time.Nanosecond*time.Duration(*responseInfo.RTT))
		child := &UnicastTransactionGraphNode{
			Key:       &key,
			Address:   responseInfo.Address,
			RTT:       responseInfo.RTT,
			RTTLegend: &rttLegend,
		}
		if responseInfo.ResponsePacket != nil {
			data.ResponseStatistics = responseInfo.ResponsePacket.ProcessingStatistics
			data.Trajectory = responseInfo.ResponsePacket.Trajectory
			child.Redundant = responseInfo.ResponsePacket.AlreadyProcessed
			if !*responseInfo.ResponsePacket.AlreadyProcessed {
				loadUnicastTransactionNode(peerAddress, transactionId, prefix, child, cache, m)
			}
		}
		child.ErrorResponse = responseInfo.Error
		data.OutboundNodes = append(data.OutboundNodes, *child)

	}
	node.Responses = &data
}

func requestUnicastNodeStatistics(peerAddress string, transactionId, nodeAddress string) *propagate.UnicastStatisticsEntry {
	defer util.CatchPanic()
	r := propagate.UnicastTransactionStatisticsResponse{}
	if peerAddress == nodeAddress {
		web.GetJson(nodeAddress+"/unicastTransactionStatistics?transactionId="+transactionId, time.Second, 200,
			nil, &r)
	} else {
		name := nodeAddress[strings.LastIndex(nodeAddress, "/"):]
		path := fmt.Sprintf("/proxy/%s/unicastTransactionStatistics", name)
		web.GetJson(peerAddress+path+"?transactionId="+transactionId, time.Second, 200,
			map[string]string{propagate.ProxyTargetAddressHeaderName: nodeAddress}, &r)
	}
	return r.Statistics
}
