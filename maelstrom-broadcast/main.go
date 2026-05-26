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
		messages: map[float64]knowers{},
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

type knowers map[string]struct{}

type server struct {
	n         *maelstrom.Node
	mu        sync.Mutex
	neighbors []string
	messages  map[float64]knowers
}

func (s *server) handleBroadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	message := body["message"].(float64)
	s.store(message, knowers{s.n.ID(): {}})
	s.gossip()

	body["type"] = "broadcast_ok"
	delete(body, "message")
	return s.n.Reply(msg, body)
}

func (s *server) handleRead(msg maelstrom.Message) error {
	body := map[string]any{"type": "read_ok"}
	s.mu.Lock()
	body["messages"] = slices.Collect(maps.Keys(s.messages))
	s.mu.Unlock()
	return s.n.Reply(msg, body)
}

func (s *server) handleTopology(msg maelstrom.Message) error {
	var body struct {
		Topology map[string][]string `json:"topology"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	s.neighbors = body.Topology[s.n.ID()]
	s.mu.Unlock()
	return s.n.Reply(msg, map[string]any{"type": "topology_ok"})
}

// store records a message and unions in the given knowers. Returns true if the
// message is new to this node.
func (s *server) store(m float64, k knowers) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.messages[m]; ok {
		maps.Copy(existing, k)
		existing[s.n.ID()] = struct{}{}
		return false
	}
	k[s.n.ID()] = struct{}{}
	s.messages[m] = k
	return true
}

type wireMessage struct {
	Message float64  `json:"message"`
	Knowers []string `json:"knowers"`
}

// gossip sends each neighbor the messages it isn't yet known to have, with the
// current knowers set piggybacked.
func (s *server) gossip() {
	s.mu.Lock()
	plan := make(map[string][]wireMessage, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		var toSend []wireMessage
		for m, k := range s.messages {
			if _, ok := k[neighbor]; ok {
				continue
			}
			toSend = append(toSend, wireMessage{Message: m, Knowers: slices.Collect(maps.Keys(k))})
		}
		if len(toSend) > 0 {
			plan[neighbor] = toSend
		}
	}
	s.mu.Unlock()

	for neighbor, toSend := range plan {
		body := map[string]any{"type": "gossip", "messages": toSend}
		s.n.RPC(neighbor, body, func(reply maelstrom.Message) error {
			if reply.RPCError() != nil {
				return nil
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, m := range toSend {
				if k, ok := s.messages[m.Message]; ok {
					k[neighbor] = struct{}{}
				}
			}
			return nil
		})
	}
}

func (s *server) handleGossip(msg maelstrom.Message) error {
	var body struct {
		Messages []wireMessage `json:"messages"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	anyNew := false
	for _, m := range body.Messages {
		k := knowers{msg.Src: {}}
		for _, n := range m.Knowers {
			k[n] = struct{}{}
		}
		if s.store(m.Message, k) {
			anyNew = true
		}
	}
	if anyNew {
		s.gossip()
	}
	return s.n.Reply(msg, map[string]any{"type": "gossip_ok"})
}
