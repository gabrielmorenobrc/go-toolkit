package rpc

import (
	"bytes"
	"io"
	"math/rand"
	"net"
	"sparrowhawktech/toolkit/util"
	"time"

	"toolkit/errors"
)

const (
	baseErrorCode            = 31000
	RemoteExceptionErrorCode = baseErrorCode + 1
)

func init() {
	const errorSpace = "toolkit.rpc"
	errors.GlobalDictionary.RegisterSpace(errorSpace, "Fanout API", baseErrorCode, baseErrorCode+999)
	errors.GlobalDictionary.RegisterError(errorSpace, RemoteExceptionErrorCode, "The remote process returned an error on excecution.")
}

const (
	ExecutionResultOk    = 0x00
	ExecutionResultError = 0x01
	key                  = "6Reoj5r8urZ5EGGU//UyXPmBHmjI3VFm7k4vn20SBBM="
	DefaultDeadline      = time.Second * 2
)

type Client struct {
	address *string
}

func (o *Client) Request(handlerFunctionName string, payload []byte, timeout ...time.Duration) []byte {
	return o.doRequest(handlerFunctionName, payload, true, timeout...)
}

func (o *Client) RequestUnsigned(handlerFunctionName string, payload []byte, timeout ...time.Duration) []byte {
	return o.doRequest(handlerFunctionName, payload, false, timeout...)
}

func (o *Client) doRequest(handlerFunctionName string, payload []byte, signed bool, timeout ...time.Duration) []byte {
	deadline := DefaultDeadline
	if len(timeout) > 0 {
		deadline = timeout[0]
	}

	msg := &bytes.Buffer{}
	if signed {
		o.createMessage(handlerFunctionName, payload, msg)
	} else {
		o.createMessageUnsigned(handlerFunctionName, payload, msg)
	}
	tcpAddr, err := net.ResolveTCPAddr("tcp", *o.address)
	util.CheckErr(err)
	connection, err := net.DialTCP("tcp", nil, tcpAddr)
	util.CheckErr(err)
	defer o.closeConnection(connection)

	err = connection.SetDeadline(time.Now().Add(deadline))
	util.CheckErr(err)
	o.writeAll(connection, msg.Bytes())
	err = connection.CloseWrite()
	util.CheckErr(err)

	response, err := io.ReadAll(connection)
	util.CheckErr(err)

	if len(response) == 0 {
		errors.ThrowError(RemoteExceptionErrorCode, "empty response")
	}

	if response[0] == ExecutionResultError {
		errors.ThrowError(RemoteExceptionErrorCode, string(response[1:]))
	}
	return response[1:]
}

func (o *Client) closeConnection(connection *net.TCPConn) {
	err := connection.Close()
	if err != nil {
		util.ProcessError(err)
	}
}

func (o *Client) writeAll(connection *net.TCPConn, out []byte) {
	offset := 0
	l := len(out)
	r := o.write(connection, out)
	offset += r
	for offset < l {
		r = o.write(connection, out[offset:])
		offset += r
	}
}

func (o *Client) write(connection *net.TCPConn, out []byte) int {
	r, err := connection.Write(out)
	util.CheckErr(err)
	return r
}

func (o *Client) createMessage(handlerFunctionName string, payload []byte, msg io.Writer) {

	id := rand.Uint64()
	idArray := u64ToByteBigEndian(id)

	timestamp := uint64(time.Now().Unix())
	timestampArray := u64ToByteBigEndian(timestamp)
	util.Write(msg, idArray)
	util.Write(msg, timestampArray)

	writeSignature(msg, id, timestamp)

	util.Write(msg, []byte(handlerFunctionName))
	util.Write(msg, []byte{0x00})

	if len(payload) > 0 {
		util.Write(msg, []byte(payload))
	}
}

func (o *Client) createMessageUnsigned(handlerFunctionName string, payload []byte, msg io.Writer) {

	util.Write(msg, []byte(handlerFunctionName))
	util.Write(msg, []byte{0x00})

	if len(payload) > 0 {
		util.Write(msg, []byte(payload))
	}
}

func NewClient(address string) *Client {
	return &Client{address: &address}
}
