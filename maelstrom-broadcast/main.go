package main

import (
	"context"
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

	// this will resend anything that never reached its destination
	// (optimistically assumed to have succeeded, but didn't)
	ticker := time.NewTicker(500 * time.Millisecond)
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

func (s *server) starTopology() {
	// s.n.NodeIDs() is guaranteed to be same nodes in same order for each node
	center := s.n.NodeIDs()[0]
	if s.n.ID() == center {
		s.neighbors = s.n.NodeIDs()[1:]
		return
	}
	s.neighbors = s.n.NodeIDs()[:1]
}

// handleTopology is called once by maelstrom on setup. By default it will use a
// grid topology. Here I keep the method as a convenient way to get the
// topology configured at the correct time during setup, but overwrite the
// actual topology with a star topology to improve performance.
func (s *server) handleTopology(msg maelstrom.Message) error {
	var body struct {
		Topology map[string][]string `json:"topology"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	// s.neighbors = body.Topology[s.n.ID()]
	s.starTopology()
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

// wireMessages stores knowers as a slice rather than a map so that it plays nice
// with JSON
type wireMessage struct {
	Message float64  `json:"message"`
	Knowers []string `json:"knowers"`
}

// gossip sends each neighbor the messages it isn't yet known to have, with the
// current knowers set piggybacked. Destination nodes are optimistically added
// to knowers. If there is an RPC error the destination node is removed from
// knowers. The ticker in main will periodically trigger gossip which will
// resend any messages not known by neighbors.
func (s *server) gossip() {
	s.mu.Lock()
	// messages to send to each neighbor
	plan := make(map[string][]wireMessage, len(s.neighbors))
	for _, neighbor := range s.neighbors {
		var toSend []wireMessage
		for m, k := range s.messages {
			// if neighbor in-the-know, do nothing
			if _, ok := k[neighbor]; ok {
				continue
			}
			// else add message to toSend
			toSend = append(toSend, wireMessage{Message: m, Knowers: slices.Collect(maps.Keys(k))})
			// add neighbor to knowers optimistically
			s.messages[m][neighbor] = struct{}{}
		}
		if len(toSend) > 0 {
			plan[neighbor] = toSend
		}
	}
	s.mu.Unlock()

	for neighbor, toSend := range plan {
		body := map[string]any{"type": "gossip", "messages": toSend}

		go func() {
			ctx, cncl := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cncl()

			_, err := s.n.SyncRPC(ctx, neighbor, body)
			// if RPC error, remove it from knowers. The ticker will fill gaps
			// caused by optimistically assuming that a node knows something
			// that never actually reaches it
			if err != nil {
				s.mu.Lock()
				for _, wm := range toSend {
					delete(s.messages[wm.Message], neighbor)
				}
				s.mu.Unlock()
			}
		}()
	}
}

// handleGossip adds all messages in the gossip to s.messages as well as their
// knowers. If there are any messages that are new to the node, a gossip is
// triggered.
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
