// Package protocol implements Chrome Native Messaging framing:
// 4-byte little-endian uint32 length + UTF-8 JSON body.
//
// Limits (Chrome docs): host → extension 1 MB; extension → host has no hard
// Chrome cap. We encode at 1 MB and decode up to 64 MB for large bookmark trees.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

const (
	MaxEncodeSize = 1024 * 1024      // 1 MB (host → extension)
	MaxDecodeSize = 64 * 1024 * 1024 // 64 MB (extension → host)
)

func Encode(msg any) ([]byte, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	if len(body) > MaxEncodeSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", len(body), MaxEncodeSize)
	}
	out := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(out[:4], uint32(len(body)))
	copy(out[4:], body)
	return out, nil
}

func Decode(buf []byte) (messages []any, remaining []byte, err error) {
	offset := 0
	for offset+4 <= len(buf) {
		msgLen := binary.LittleEndian.Uint32(buf[offset : offset+4])
		if msgLen > MaxDecodeSize {
			return nil, buf[offset:], fmt.Errorf("message too large: %d bytes", msgLen)
		}
		total := 4 + int(msgLen)
		if offset+total > len(buf) {
			break
		}
		var msg any
		if err := json.Unmarshal(buf[offset+4:offset+total], &msg); err != nil {
			return nil, buf[offset:], err
		}
		messages = append(messages, msg)
		offset += total
	}
	return messages, buf[offset:], nil
}
