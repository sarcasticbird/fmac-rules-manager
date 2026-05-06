package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const maxMessageSize = 1024 * 1024 // 1MB

type Request struct {
	Cmd           string     `json:"cmd"`
	Site          string     `json:"site,omitempty"`
	UserContextID int        `json:"userContextId,omitempty"`
	NeverAsk      *bool      `json:"neverAsk,omitempty"`
	Rules         []RuleSpec `json:"rules,omitempty"`
	Mode          string     `json:"mode,omitempty"`
}

type RuleSpec struct {
	Site          string `json:"site"`
	UserContextID int    `json:"userContextId"`
	NeverAsk      bool   `json:"neverAsk"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading length: %w", err)
	}
	if length > maxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("reading payload: %w", err)
	}
	return buf, nil
}

func writeMessage(w io.Writer, resp Response) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("writing length: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}
