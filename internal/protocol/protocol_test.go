package protocol

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestEncodeNativeMessage(t *testing.T) {
	msg := map[string]any{"type": "ping", "requestId": "123"}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(msg)
	if binary.LittleEndian.Uint32(encoded[:4]) != uint32(len(body)) {
		t.Fatalf("header length = %d, want %d", binary.LittleEndian.Uint32(encoded[:4]), len(body))
	}
	if string(encoded[4:]) != string(body) {
		t.Fatalf("body = %s, want %s", encoded[4:], body)
	}
}

func TestDecodeOneCompleteMessage(t *testing.T) {
	msg := map[string]any{"type": "pong", "requestId": "456"}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	messages, remaining, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("got %d messages", len(messages))
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining %d bytes", len(remaining))
	}
	got := messages[0].(map[string]any)
	if got["type"] != "pong" || got["requestId"] != "456" {
		t.Fatalf("decoded %+v", got)
	}
}

func TestDecodePartialData(t *testing.T) {
	msg := map[string]any{"type": "ping", "requestId": "789"}
	encoded, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	partial := encoded[:6]
	messages, remaining, err := Decode(partial)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("got %d messages, want 0", len(messages))
	}
	if len(remaining) != 6 {
		t.Fatalf("remaining %d, want 6", len(remaining))
	}
}

func TestDecodeTwoConcatenatedMessages(t *testing.T) {
	msg1 := map[string]any{"type": "ping", "requestId": "1"}
	msg2 := map[string]any{"type": "pong", "requestId": "2"}
	a, _ := Encode(msg1)
	b, _ := Encode(msg2)
	combined := append(append([]byte{}, a...), b...)
	messages, remaining, err := Decode(combined)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("got %d messages", len(messages))
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining %d", len(remaining))
	}
	if messages[0].(map[string]any)["requestId"] != "1" {
		t.Fatalf("first = %+v", messages[0])
	}
	if messages[1].(map[string]any)["requestId"] != "2" {
		t.Fatalf("second = %+v", messages[1])
	}
}

func TestEncodeRejectsOversized(t *testing.T) {
	big := make([]byte, MaxEncodeSize+1)
	for i := range big {
		big[i] = 'a'
	}
	_, err := Encode(map[string]any{"x": string(big)})
	if err == nil {
		t.Fatal("expected error")
	}
}
