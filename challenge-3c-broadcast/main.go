package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	s := newServer(n)

	n.Handle("broadcast", s.broadcastHandler)
	n.Handle("forward", s.forwardHandler)
	n.Handle("read", s.readHandler)
	n.Handle("topology", s.topologyHandler)
	n.Handle("gossip", s.gossipHandler)

	go runPeriodically(100*time.Millisecond, s.gossip)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n *maelstrom.Node

	messages map[int]bool
}

type Payload struct {
	MessageType string `json:"type"`
	Message     int    `json:"message"`
	Messages    []int  `json:"messages"`
}

func newServer(n *maelstrom.Node) *server {
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

func (s *server) otherNodeIDs() []string {
	nodes := s.n.NodeIDs()
	var others []string
	for _, node := range nodes {
		if node != s.n.ID() {
			others = append(others, node)
		}
	}
	return others
}

func (s *server) gossip() {
	others := s.otherNodeIDs()
	if len(others) == 0 {
		return // no others to gossip to rn
	}
	randomOther := others[rand.Intn(len(others))]

	// send all messages to that node
	body := map[string]any{
		"type":     "gossip",
		"messages": s.messages,
	}
	if err := s.n.Send(randomOther, body); err != nil {
		return
	}
}

func (s *server) gossipHandler(msg maelstrom.Message) error {
	// get union of my messages and incoming messages
	body, err := unmarshalBody(msg)
	if err != nil {
		return err
	}
	incomingMessages := body["messages"].(map[int]bool)
	u := union(s.messages, incomingMessages)
	s.messages = u
	return nil
}

func union(s1, s2 map[int]bool) map[int]bool {
	u := make(map[int]bool)
	for e := range s1 {
		u[e] = true
	}
	for e := range s2 {
		u[e] = true
	}
	return u
}

func (s *server) forwardHandler(msg maelstrom.Message) error {
	return s.recordMessage(msg)
}

func (s *server) readHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type":     "read_ok",
		"messages": ToSlice(s.messages),
	})
}

// Placeholder
func (s *server) topologyHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type": "topology_ok",
	})
}

func (s *server) forward(msg maelstrom.Message) error {
	body := unmarshalPayload(msg)
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

func unmarshalPayload(msg maelstrom.Message) Payload {
	var p Payload
	json.Unmarshal(msg.Body, &p)
	return p
}

func (s *server) recordMessage(msg maelstrom.Message) error {
	body := unmarshalPayload(msg)
	s.messages[body.Message] = true
	return nil
}

func ToSlice(m map[int]bool) []int {
	keys := make([]int, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}

func runPeriodically(d time.Duration, f func()) {
	for {
		time.Sleep(d)
		f()
	}
}
