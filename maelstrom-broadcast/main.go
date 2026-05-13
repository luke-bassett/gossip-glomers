package main

import (
	"encoding/json"
	"log"
	"maps"
	"slices"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	s := &server{n: maelstrom.NewNode(), messages: map[float64]struct{}{}}
	s.n.Handle("broadcast", s.handleBroadcast)
	s.n.Handle("read", s.handleRead)
	s.n.Handle("topology", s.handleTopology)
	s.n.Handle("gossip", s.handleGossip)

	if err := s.n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n         *maelstrom.Node
	mu        sync.Mutex
	messages  map[float64]struct{}
	neighbors []string
}

func (s *server) handleBroadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message := body["message"].(float64)
	s.storeAndGossip(message, msg.Src)

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
	s.neighbors = body.Topology[s.n.ID()]
	s.mu.Unlock()
	return s.n.Reply(msg, reply_body)
}

func (s *server) handleGossip(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message := body["message"].(float64)
	s.storeAndGossip(message, msg.Src)

	return nil
}

func (s *server) storeAndGossip(message float64, except string) {

	s.mu.Lock()
	if _, ok := s.messages[message]; ok {
		s.mu.Unlock()
		return
	}

	// store
	s.messages[message] = struct{}{}
	neighbors := append([]string(nil), s.neighbors...)
	s.mu.Unlock()

	// gossip
	body := map[string]any{"message": message, "type": "gossip"}
	for _, neighbor := range neighbors {
		if neighbor != except {
			s.n.Send(neighbor, body)
		}
	}
}
