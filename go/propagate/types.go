package propagate

import (
	"encoding/json"
	"sync"
	"time"
)

type PacketKind int

const (
	SystemPacketKind = PacketKind(1)
	UserPacketKind   = PacketKind(2)
)

type DebugInstructions struct {
	RelayFailureDepth   *int  `json:"relayFailureDepth"`   // produce an exception at the nth node upon trying to forward the packet
	ProcessFailureDepth *int  `json:"processFailureDepth"` // produce an exception at the nth node upon executing the topic handler for the packet
	Trajectory          *bool `json:"trajectory"`          // Include trajectory in response packets
}

type RequestPacket struct {
	PacketKind        PacketKind         `json:"packetKind"`
	TransactionId     *string            `json:"transactionId"`
	Recipient         *string            `json:"recipient"`
	Timestamp         *time.Time         `json:"timestamp"`
	Topic             *string            `json:"topic"`
	Payload           []byte             `json:"payload"`
	Trajectory        []string           `json:"trajectory"`
	DebugInstructions *DebugInstructions `json:"debugInstructions"`
}

type UnicastProcessingStatistics struct {
	ProcessTime     *int64  `json:"processTime"`
	TotalTime       *int64  `json:"totalTime"`
	ProcessingError *string `json:"processingError"`
	ForwardingError *string `json:"forwardingError"`
}

type UnicastResponseStatistics struct {
	RTT           *int64
	ResponseGraph map[string]UnicastResponseStatistics
}

type UnicastResponsePacket struct {
	TransactionId        *string                      `json:"transactionId"`
	Address              *string                      `json:"address"`
	AlreadyProcessed     *bool                        `json:"alreadyProcessed"`
	Successful           *bool                        `json:"successful"`
	Payload              []byte                       `json:"payload"`
	ProcessingStatistics *UnicastProcessingStatistics `json:"processingStatistics"`
	Trajectory           []string
}

type UnicastResponseInfo struct {
	RTT            *int64
	ResponsePacket *UnicastResponsePacket
	Address        *string
	Error          *string
}

type UnicastProcessingContext struct {
	mux             sync.Mutex
	RequestPacket   *RequestPacket
	Statistics      *UnicastProcessingStatistics
	ResponseInfoMap map[string]UnicastResponseInfo
	ResponsePacket  *UnicastResponsePacket
}

func (o *UnicastProcessingContext) PutResponseInfo(key string, info UnicastResponseInfo) {
	o.mux.Lock()
	defer o.mux.Unlock()
	o.ResponseInfoMap[key] = info
}

type BroadcastResponsePacket struct {
	TransactionId    *string  `json:"transactionId"`
	Address          *string  `json:"address"`
	AlreadyProcessed *bool    `json:"alreadyProcessed"`
	Trajectory       []string `json:"trajectory"`
}

type BroadcastProcessingStatistics struct {
	ProcessTime       *int64  `json:"processTime"`
	TotalTime         *int64  `json:"totalTime"`
	BroadcastError    *string `json:"broadcastError"`
	TopicHandlerError *string `json:"topicHandlerError"`
}

type BroadcastResponseInfo struct {
	GatewayAddress *string                  `json:"gatewayAddress"`
	TransactionId  *string                  `json:"transactionId"`
	ResponsePacket *BroadcastResponsePacket `json:"responsePacket"`
	RTT            *int64                   `json:"rtt"`
	Error          *string                  `json:"error"`
}

type BroadcastProcessingContext struct {
	RequestPacket      *RequestPacket
	Statistics         *BroadcastProcessingStatistics
	GatewaysStatistics map[string]BroadcastResponseInfo
}

type BroadcastResults struct {
	Statistics         *BroadcastProcessingStatistics   `json:"statistics"`
	GatewaysStatistics map[string]BroadcastResponseInfo `json:"gatewaysStatistics"`
	TransactionId      *string                          `json:"transactionId"`
}

type ExecuteRequestResults struct {
	Payload            []byte                         `json:"payload"`
	Statistics         *UnicastProcessingStatistics   `json:"statistics"`
	ResponseStatistics map[string]UnicastResponseInfo `json:"responseStatistics"`
	TransactionId      *string                        `json:"transactionId"`
}

type UserData struct {
	TypeKey *int            `json:"typeKey"`
	Data    json.RawMessage `json:"data"`
}

type NodeInfo struct {
	Address      *string
	PrimaryPeers []string
	UserData     *UserData
}

type AnnounceRequest struct {
	NodeInfo *NodeInfo
	Members  map[string]NodeInfo
}

type RTT struct {
	Recipient string
	Gateway   string
	Duration  time.Duration
}
