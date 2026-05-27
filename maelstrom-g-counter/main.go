package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	s := newServer()
	s.n.Handle("add", s.handleAdd)
	s.n.Handle("read", s.handleRead)

	if err := s.n.Run(); err != nil {
		log.Fatal(err)
	}
}

func newServer() *server {
	n := maelstrom.NewNode()
	kv := maelstrom.NewSeqKV(n) // client to shared Sequential KV store
	return &server{
		n:  n,
		kv: kv,
	}
}

type server struct {
	n  *maelstrom.Node
	kv *maelstrom.KV
}

// handleAdd adds the delta to the node's key in the SeqKV. CompareAndSwap is
// used to avoid race conditions rather than mutexes. The value is checked and
// then confirmed when writing. If it's not the expected value just try again.
func (s *server) handleAdd(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	delta := int(body["delta"].(float64))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for {
		// get current
		current, err := s.readCount(ctx, s.n.ID())
		if err != nil {
			return err
		}
		err = s.kv.CompareAndSwap(ctx, s.n.ID(), current, current+delta, true)

		// update success
		if err == nil {
			break
		}
		// lost the race
		if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.PreconditionFailed {
			continue
		}
		// something bad happened
		return err
	}

	return s.n.Reply(msg, map[string]any{"type": "add_ok"})
}

func (s *server) readCount(ctx context.Context, key string) (int, error) {
	n, err := s.kv.ReadInt(ctx, key)
	if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.KeyDoesNotExist {
		return 0, nil
	}
	return n, err
}

// handleRead returns the current value of the global counter, i.e., the sum of
// each node's counter. In a real-world scenario there may be some reads that
// can tolerate staleness which would only need the sequentially correct
// session-local values, and other reads that need the globally current value.
// For this challenge we are only interested in the latter case. To achieve that
// with SeqKV we need to write something to force a global synchronization up to
// the point in time of the write, and then read at that point in time.
func (s *server) handleRead(msg maelstrom.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// forces this session forward in the SeqKV global order
	if err := s.kv.CompareAndSwap(ctx, "dummy", 0, 0, true); err != nil {
		return err
	}

	var total int
	for _, id := range s.n.NodeIDs() {
		count, err := s.readCount(ctx, id)
		if err != nil {
			return err
		}
		total += count
	}
	body := map[string]any{"type": "read_ok", "value": total}
	return s.n.Reply(msg, body)
}
