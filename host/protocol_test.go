package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestReadMessage(t *testing.T) {
	msg := map[string]string{"cmd": "list"}
	payload, _ := json.Marshal(msg)

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)

	result, err := readMessage(&buf)
	if err != nil {
		t.Fatalf("readMessage error: %v", err)
	}

	var parsed map[string]string
	json.Unmarshal(result, &parsed)
	if parsed["cmd"] != "list" {
		t.Errorf("expected cmd=list, got %s", parsed["cmd"])
	}
}

func TestWriteMessage(t *testing.T) {
	resp := Response{OK: true, Data: map[string]any{"count": 5}}

	var buf bytes.Buffer
	err := writeMessage(&buf, resp)
	if err != nil {
		t.Fatalf("writeMessage error: %v", err)
	}

	// Read the length prefix
	var length uint32
	binary.Read(&buf, binary.LittleEndian, &length)

	// Read the JSON payload
	payload := make([]byte, length)
	buf.Read(payload)

	var parsed Response
	json.Unmarshal(payload, &parsed)
	if !parsed.OK {
		t.Error("expected ok=true")
	}
}

func TestReadMessageTooLarge(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(2*1024*1024))
	buf.Write(make([]byte, 100))

	_, err := readMessage(&buf)
	if err == nil {
		t.Error("expected error for oversized message")
	}
}
