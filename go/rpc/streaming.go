package rpc

import (
	"io"
	"math/rand"
	"net"
	"sparrowhawktech/toolkit/util"
	"time"

	"toolkit/errors"
)

type RequestConnection struct {
	connection *net.TCPConn
	writing    bool
}

func (o *RequestConnection) StartWrite() io.Writer {
	return o.connection
}

func (o *RequestConnection) CloseWrite() io.Reader {
	if !o.writing {
		panic("Write channel already closed")
	}
	util.CheckErr(o.connection.CloseWrite())
	b := []byte{0}
	util.Read(o.connection, b) // replace by util.Read once merged
	// util.Log("repl").Printf("Incoming payload size: %d", len(b))
	if b[0] == ExecutionResultError {
		content, err := io.ReadAll(o.connection)
		util.CheckErr(err)
		errors.ThrowError(RemoteExceptionErrorCode, string(content))
	}
	return o.connection
}

func (o *RequestConnection) Close() {
	defer util.Close(o.connection)
	_, err := io.ReadAll(o.connection)
	if err != nil {
		util.Log("warning").Printf("Error flushing tcp buffers: %s", util.ResolveErrorMessage(err))
	}
}

type StreamingClient struct {
	address *string
}

func (o *StreamingClient) Request(handlerFunctionName string, timeout time.Duration) *RequestConnection {
	tcpAddr, err := net.ResolveTCPAddr("tcp", *o.address)
	util.CheckErr(err)
	connection, err := net.DialTCP("tcp", nil, tcpAddr)
	util.CheckErr(err)
	rc := RequestConnection{connection: connection, writing: true}
	defer func() {
		if r := recover(); r != nil {
			rc.Close()
			panic(r)
		}
	}()

	err = connection.SetDeadline(time.Now().Add(timeout))
	util.CheckErr(err)

	o.writeHeader(handlerFunctionName, connection)
	return &rc
}

func (o *StreamingClient) writeHeader(handlerFunctionName string, msg io.Writer) {

	id := rand.Uint64()
	idArray := u64ToByteBigEndian(id)

	timestamp := uint64(time.Now().Unix())
	timestampArray := u64ToByteBigEndian(timestamp)
	util.Write(msg, idArray)
	util.Write(msg, timestampArray)

	writeSignature(msg, id, timestamp)

	util.Write(msg, []byte(handlerFunctionName))
	util.Write(msg, []byte{0x00})
}

func NewStreamingClient(address string) *StreamingClient {
	return &StreamingClient{address: &address}
}
