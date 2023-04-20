package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
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

func TestServerRecordMessage(t *testing.T) {
	n := maelstrom.NewNode()
	s := newServer(n)

	messages := []int{42, 88, 100}
	for _, m := range messages {
		msg := buildMessage(map[string]any{"message": m})
		if err := s.recordMessage(msg); err != nil {
			t.Errorf("recordMessage failed with error %v:", err)
		}
		if s.messages[m] != true {
			t.Errorf("recordMessage failed to add message %v to map %v", m, s.messages)
		}
	}
	if len(s.messages) != len(messages) {
		t.Errorf("recordMessage failed to add all messages to map %v", s.messages)
	}
}

func TestServerBroadcastHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard
	s := newServer(n)

	msg := buildMessage(map[string]any{"type": "broadcast", "message": 42})
	if err := s.broadcastHandler(msg); err != nil {
		t.Errorf("broadcastHandler failed with error %v:", err)
	}
}

// func TestForward(t *testing.T) {
// 	n := maelstrom.NewNode()
// 	n.Stdout = io.Discard // suppress all output
// 	ids := []string{"a", "b", "c"}
// 	n.Init("a", ids)
// 	s := newServer(n)

// 	msg := buildMessage(map[string]any{"type": "forward", "message": 42})

// 	// should send messages to b and c
// 	rawOutput, err := s.captureOutput(func() error { return s.forward(msg) })
// 	if err != nil {
// 		t.Errorf("forward failed with error %v:", err)
// 	}

// 	outputs := strings.Split(strings.TrimSpace(rawOutput), "\n")
// 	if l := len(outputs); l != 2 {
// 		t.Errorf("Should have gotten 2 outputs, got %v", l)
// 	}
// 	for i, output := range outputs {
// 		expected := fmt.Sprintf(`{"src":"a","dest":"%v","body":{"message":42,"type":"forward"}}`, ids[i+1])
// 		if !strings.Contains(output, expected) {
// 			t.Errorf("Output '%v' doesn't contain expected substring '%v'", output, expected)
// 		}
// 	}
// }

func TestServerForwardHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard // suppress all output
	s := newServer(n)

	msg := buildMessage(map[string]any{"type": "forward", "message": 42})
	if err := s.forwardHandler(msg); err != nil {
		t.Errorf("forwardHandler failed with error %v:", err)
	}
}

func TestServerReadHandler(t *testing.T) {
	n := maelstrom.NewNode()
	s := newServer(n)
	s.messages = map[int]bool{42: true, 88: true, 100: true}

	msg := buildMessage(map[string]any{"type": "read"})

	output, err := s.captureOutput(func() error { return s.readHandler(msg) })
	if err != nil {
		t.Errorf("readHandler failed with error %v:", err)
	}
	type readResponse struct {
		Body struct {
			Type      string `json:"type"`
			Messages  []int  `json:"messages"`
			InReplyTo int    `json:"in_reply_to"`
		} `json:"body"`
	}
	var r readResponse
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		t.Errorf("readHandler output is not valid JSON: %v", strings.TrimSpace(output))
	}
	sort.Ints(r.Body.Messages)
	if r.Body.Type != "read_ok" {
		t.Errorf("readHandler output is not correct: %v", strings.TrimSpace(output))
	}
	if !reflect.DeepEqual(r.Body.Messages, []int{42, 88, 100}) {
		t.Errorf("readHandler output is not correct: %v", strings.TrimSpace(output))
	}
}

func TestServerTopologyHandler(t *testing.T) {
	n := maelstrom.NewNode()
	n.Stdout = io.Discard
	s := newServer(n)

	msg := buildMessage(map[string]any{"type": "topology"})

	if err := s.topologyHandler(msg); err != nil {
		t.Errorf("topologyHandler failed with error %v:", err)
	}
}

func buildMessage(body map[string]any) maelstrom.Message {
	bodyEnc, _ := json.Marshal(body)
	return maelstrom.Message{Body: bodyEnc}
}

func (s *server) captureOutput(handlerFunc func() error) (string, error) {
	var buf bytes.Buffer
	s.n.Stdout = &buf
	defer func() { s.n.Stdout = os.Stdout }()
	err := handlerFunc()
	return buf.String(), err
}

func TestToSlice(t *testing.T) {
	want := ToSlice(map[int]bool{1: true, 7: false})
	sort.Ints(want)
	got := []int{1, 7}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestUnion(t *testing.T) {
	s1 := map[int]bool{1: true, 3: true}
	s2 := map[int]bool{0: true, 1: true}
	want := map[int]bool{0: true, 1: true, 3: true}
	got := union(s1, s2)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("wanted %v, got %v", want, got)
	}
}
