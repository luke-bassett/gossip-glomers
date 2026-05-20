package main

import (
	"encoding/json"
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	s := &server{
		n:        maelstrom.NewNode(),
		messages: map[float64]struct{}{},
		outbox:   map[string]map[float64]struct{}{},
	}
	s.n.Handle("broadcast", s.handleBroadcast)
	s.n.Handle("read", s.handleRead)
	s.n.Handle("topology", s.handleTopology)
	s.n.Handle("gossip", s.handleGossip)

	ticker := time.NewTicker(100 * time.Millisecond)
	go func() {
		for range ticker.C {
			s.gossip()
		}
	}()

	if err := s.n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n        *maelstrom.Node
	mu       sync.Mutex
	messages map[float64]struct{}
	outbox   map[string]map[float64]struct{}
}

func (s *server) handleBroadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message := body["message"].(float64)
	s.storeAndGossip(msg.Src, message)

	body["type"] = "broadcast_ok"
	delete(body, "message")
	return s.n.Reply(msg, body)
}

func (s *server) handleRead(msg maelstrom.Message) error {

	body := map[string]any{}
	body["type"] = "read_ok"
	s.mu.Lock()
	body["messages"] = slices.Collect(maps.Keys(s.messages))
	s.mu.Unlock()
	return s.n.Reply(msg, body)
}

func (s *server) handleTopology(msg maelstrom.Message) error {
	var body struct {
		Type     string              `json:"type"`
		Topology map[string][]string `json:"topology"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	reply_body := map[string]any{"type": "topology_ok"}
	s.mu.Lock()
	for _, neighbor := range body.Topology[s.n.ID()] {
		s.outbox[neighbor] = make(map[float64]struct{})
	}
	s.mu.Unlock()
	return s.n.Reply(msg, reply_body)
}

func (s *server) storeAndGossip(source string, message float64) {
	s.store(source, message)
	s.gossip()
}

// store records a message in the node's set and adds it to every neighbor's
// outbox except source. Skips duplicates.
func (s *server) store(source string, message float64) {
	s.mu.Lock()
	// do nothing if we've seen it
	if _, ok := s.messages[message]; ok {
		s.mu.Unlock()
		return
	}
	// store the message
	s.messages[message] = struct{}{}

	// add to outboxes
	for neighbor := range s.outbox {
		if source != neighbor {
			s.outbox[neighbor][message] = struct{}{}
		}
	}
	s.mu.Unlock()
}

// gossip sends each neighbor the entire contents of its outbox in a single RPC.
// Ack drains the outbox with handleGossipOk.
func (s *server) gossip() {
	s.mu.Lock()
	outboxSnapshot := make(map[string][]float64, len(s.outbox))
	for neighbor, messages := range s.outbox {
		if len(messages) > 0 {
			outboxSnapshot[neighbor] = slices.Collect(maps.Keys(messages))
		}
	}
	s.mu.Unlock()

	for neighbor, messages := range outboxSnapshot {
		body := map[string]any{"type": "gossip", "messages": messages}
		// send the messages and handle replys with handleGossipAck
		s.n.RPC(neighbor, body, s.handleGossipOk)
	}
}

// handleGossip stores incoming messages and regossips them.
func (s *server) handleGossip(msg maelstrom.Message) error {
	var body struct {
		Messages []float64 `json:"messages"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	for _, message := range body.Messages {
		s.storeAndGossip(msg.Src, message)
	}
	reply_body := map[string]any{"type": "gossip_ok", "messages": body.Messages}
	return s.n.Reply(msg, reply_body)
}

// handleGossipOk removes acked messages from the outbox.
func (s *server) handleGossipOk(msg maelstrom.Message) error {
	var body struct {
		Messages []float64 `json:"messages"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	for _, message := range body.Messages {
		s.mu.Lock()
		delete(s.outbox[msg.Src], message)
		s.mu.Unlock()
	}
	return nil
}
