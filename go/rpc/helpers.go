package rpc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"sparrowhawktech/toolkit/util"
)

func u64ToByteBigEndian(u64 uint64) []byte {

	u64AsByte := make([]byte, 8)
	binary.BigEndian.PutUint64(u64AsByte, uint64(u64))
	return u64AsByte
}

func writeSignature(msg io.Writer, id uint64, timestamp uint64) {

	k := []byte(key)
	h := hmac.New(sha256.New, k)
	content := fmt.Sprintf("%d-%d", id, timestamp)

	_, err := h.Write([]byte(content))
	util.CheckErr(err)

	hSum := h.Sum(nil)
	u64AsByte := make([]byte, 8)
	binary.BigEndian.PutUint64(u64AsByte, uint64(len(hSum)))
	l, err := msg.Write(u64AsByte)
	util.CheckErr(err)
	if l != len(u64AsByte) {
		panic(fmt.Sprintf("written length does not match: %d vs %d", l, len(u64AsByte)))
	}
	util.Write(msg, hSum)
}
