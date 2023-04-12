package main

import (
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	s := NewServer(n)

	n.Handle("broadcast", s.broadcastHandler)
	n.Handle("forward", s.forwardHandler)
	n.Handle("read", s.readHandler)
	n.Handle("topology", s.topologyHandler)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n *maelstrom.Node

	messages map[int]bool
}

func NewServer(n *maelstrom.Node) *server {
	return &server{n: n, messages: make(map[int]bool)}
}

func (s *server) broadcastHandler(msg maelstrom.Message) error {
	if err := s.recordMessage(msg); err != nil {
		return err
	}
	if err := s.forward(msg); err != nil {
		return err
	}
	return s.n.Reply(msg, map[string]any{
		"type": "broadcast_ok",
	})
}

func (s *server) forward(msg maelstrom.Message) error {
	body, err := unmarshalBody(msg)
	if err != nil {
		return err
	}
	for _, id := range s.n.NodeIDs() {
		if id != s.n.ID() {
			if err := s.n.Send(id, body); err != nil {
				return err
			}
		}
	}
	return nil
}

func unmarshalBody(msg maelstrom.Message) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *server) forwardHandler(msg maelstrom.Message) error {
	return s.recordMessage(msg)
}

// recordMessage records a message in the server's messages map.
func (s *server) recordMessage(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.messages[int(body["message"].(float64))] = true
	return nil
}

func (s *server) readHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type":     "read_ok",
		"messages": keys(s.messages),
	})
}

func (s *server) topologyHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type": "topology_ok",
	})
}

// Keys returns the keys of a map as a slice.
func keys(m map[int]bool) []int {
	keys := make([]int, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}
