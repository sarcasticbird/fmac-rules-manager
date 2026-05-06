package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stderr)

	msg, err := readMessage(os.Stdin)
	if err != nil {
		log.Fatalf("reading message: %v", err)
	}

	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		resp := Response{OK: false, Error: "invalid JSON request"}
		writeMessage(os.Stdout, resp)
		return
	}

	resp := dispatch(req)

	if err := writeMessage(os.Stdout, resp); err != nil {
		log.Fatalf("writing response: %v", err)
	}
}
