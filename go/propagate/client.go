package propagate

import (
	"fmt"
	"time"

	"toolkit/dispatch"
	"toolkit/web"
)

const (
	BrentsUniversalKConstant   = time.Millisecond * 300
	ConnectionErrorEventName   = "propagation.connectionError"
	BroadcastResourceName      = "/broadcast"
	ExecuteRequestResourceName = "/executeRequest"
)

type ConnectionErrorEvent struct {
	TargetAddress *string
	Error         interface{}
	RequestPacket RequestPacket
}

type Client struct {
	id         *string
	dispatcher *dispatch.Dispatcher
}

func (o *Client) requestTo(address string, packet RequestPacket, timeout time.Duration) UnicastResponsePacket {
	responsePacket := UnicastResponsePacket{}
	o.doRequestTo(address, ExecuteRequestResourceName, packet, &responsePacket, timeout)
	return responsePacket
}

func (o *Client) sendBroadcastTo(address string, packet RequestPacket, timeout time.Duration) BroadcastResponsePacket {
	defer func() {
		if r := recover(); r != nil {
			o.dispatcher.Dispatch(ConnectionErrorEventName, ConnectionErrorEvent{
				TargetAddress: &address,
				Error:         r,
			})
			panic(r)
		}
	}()

	responsePacket := BroadcastResponsePacket{}
	o.doRequestTo(address, BroadcastResourceName, packet, &responsePacket, timeout)
	return responsePacket
}

func (o *Client) doRequestTo(address string, command string, packet RequestPacket, responsePacket interface{}, timeout time.Duration) {
	if address == *o.id {
		panic(fmt.Sprintf("BUG: Destination is same as self: %s", address))
	}
	packet.Trajectory = append(packet.Trajectory, *o.id)
	url := fmt.Sprintf("%s%s", address, command)
	web.PostJson(url, packet, &responsePacket, 200, timeout, nil)
}

func NewClient(id string, dispatcher *dispatch.Dispatcher) *Client {
	return &Client{
		id:         &id,
		dispatcher: dispatcher,
	}
}
