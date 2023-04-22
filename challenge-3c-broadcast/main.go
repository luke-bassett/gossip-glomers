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
	s := &server{n: n}

	n.Handle("broadcast", s.broadcastHandler) // record single value, send to others, awk
	n.Handle("share", s.shareHandler)         // update values to be a union of known and learned
	n.Handle("read", s.readHandler)           // respond with all values
	n.Handle("topology", s.topologyHandler)   // awk

	go s.gossip(100 * time.Millisecond)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n *maelstrom.Node

	values []int
}

type Body struct {
	Type   string `json:"type"`
	Value  int    `json:"message,omitempty"` // "message" overloaded, I'll use "Value"
	Values []int  `json:"messages,omitempty"`
}

func (s *server) gossip(d time.Duration) {
	for {
		time.Sleep(d)
		others := others(s.n.NodeIDs(), s.n.ID())
		if len(others) != 0 {
			randomOther := others[rand.Intn(len(others))]
			s.share([]string{randomOther}, s.values)
		}
	}
}

// Share values with nodes. Receiving nodes save a union of their current values
// and the incoming values.
func (s *server) share(destNodeIDs []string, values []int) {
	for _, id := range destNodeIDs {
		s.n.Send(id, map[string]any{
			"type":     "share",
			"messages": values,
		})
	}
}

func (s *server) broadcastHandler(msg maelstrom.Message) error {
	mb := Body{}
	if err := json.Unmarshal(msg.Body, &mb); err != nil {
		log.Fatal(err)
	}
	s.values = append(s.values, mb.Value)

	// TODO gossip one message to all others
	s.share(others(s.n.NodeIDs(), s.n.ID()), []int{mb.Value})

	return s.n.Reply(msg, map[string]any{
		"type": "broadcast_ok",
	})
}

func (s *server) shareHandler(msg maelstrom.Message) error {
	mb := Body{}
	if err := json.Unmarshal(msg.Body, &mb); err != nil {
		log.Fatal(err)
	}
	incoming := sliceToHashset(mb.Values)
	existing := sliceToHashset(s.values)
	s.values = hashsetToSlice(hashsetUnion(incoming, existing))

	return nil
}

func (s *server) readHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type":     "read_ok",
		"messages": s.values,
	})
}

func (s *server) topologyHandler(msg maelstrom.Message) error {
	return s.n.Reply(msg, map[string]any{
		"type": "topology_ok",
	})
}

func hashsetToSlice(hashset map[int]bool) []int {
	slice := make([]int, 0, len(hashset))
	for k := range hashset {
		slice = append(slice, k)
	}
	return slice
}

func sliceToHashset(slice []int) map[int]bool {
	hashset := make(map[int]bool, len(slice))
	for _, v := range slice {
		hashset[v] = true
	}
	return hashset
}

func hashsetUnion(hashset1, hashset2 map[int]bool) map[int]bool {
	union := make(map[int]bool)
	for k, v := range hashset1 {
		union[k] = v
	}
	for k, v := range hashset2 {
		union[k] = v
	}
	return union
}

func others(allValues []string, excluded string) []string {
	var results []string
	for _, v := range allValues {
		if v != excluded {
			results = append(results, v)
		}
	}
	return results
}
