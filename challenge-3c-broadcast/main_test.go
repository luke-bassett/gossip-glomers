package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// Supress log output during testing
// https://golangcode.com/disable-log-output-during-tests/
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestServerBroadcastHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard
	s := &server{n: n}

	messages := []int{42, 88, 100}
	for i, m := range messages {
		msg := buildMessage(map[string]any{"type": "broadcast", "message": m})
		if err := s.broadcastHandler(msg); err != nil {
			t.Errorf("broadcastHandler failed with error %v:", err)
		}
		if len(s.messages) != i+1 || s.messages[i] != messages[i] {
			t.Errorf("broadcastHandler failed to append message to slice")
		}
	}
}

func TestGossip(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard // suppress all output
	ids := []string{"a", "b", "c"}
	n.Init("a", ids)
	s := &server{n: n}

	if err := s.gossip(42); err != nil {
		t.Errorf("gossip failed with error %v:", err)
	}

	// should send messages to b and c
	rawOutput := captureOutput(func() { s.gossip(42) })
	outputs := strings.Split(strings.TrimSpace(rawOutput), "\n")
	if l := len(outputs); l != 2 {
		t.Errorf("Should have gotten 2 outputs, got %v", l)
	}
	for i, output := range outputs {
		expected := fmt.Sprintf(`{"src":"a","dest":"%v","body":{"message":42,"type":"gossip"}}`, ids[i+1])
		if !strings.Contains(output, expected) {
			t.Errorf("Output '%v' doesn't contain expected substring '%v'", output, expected)
		}
	}

}

func TestServerGossipHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard // suppress all output
	s := &server{n: n}

	messages := []int{42, 88, 100}
	for i, m := range messages {
		msg := buildMessage(map[string]any{"type": "gossip", "message": m})
		if err := s.gossipHandler(msg); err != nil {
			t.Errorf("gossipHandler failed with error %v:", err)
		}
		if len(s.messages) != i+1 || s.messages[i] != messages[i] {
			t.Errorf("gossipHandler failed to append message to slice")
		}
	}
}

func TestServerReadHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard
	s := &server{n: n, messages: []int{42, 88, 100}}

	msg := buildMessage(map[string]any{"type": "read"})

	output := captureOutput(func() { s.readHandler(msg) })
	expected := "{\"body\":{\"in_reply_to\":0,\"messages\":[42,88,100],\"type\":\"read_ok\"}}"

	if !strings.Contains(output, expected) {
		t.Errorf("Sent incorrect reply '%v', it should contain '%v'", output, expected)
	}
}

func TestServerTopologyHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard
	s := &server{n: n, messages: []int{42, 88, 100}}

	msg := buildMessage(map[string]any{"type": "topology"})

	if err := s.topologyHandler(msg); err != nil {
		t.Errorf("topologyHandler failed with error %v:", err)
	}

}

func buildMessage(body map[string]any) maelstrom.Message {
	bodyEnc, _ := json.Marshal(body)
	return maelstrom.Message{Body: bodyEnc}
}

func captureOutput(handlerFunc func()) string {
	var buf bytes.Buffer
	output := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(output)
	handlerFunc()
	return buf.String()
}
