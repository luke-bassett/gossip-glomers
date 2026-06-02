package main

import (
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n  *maelstrom.Node
	mu sync.Mutex
	kv map[int]int
}

func newServer() *server {
	n := maelstrom.NewNode()
	kv := map[int]int{}
	return &server{n: n, kv: kv}
}

func main() {
	s := newServer()
	s.n.Handle("txn", s.handleTxn)
	if err := s.n.Run(); err != nil {
		log.Fatal(err)
	}
}

type txnBody struct {
	ID  int     `json:"msg_id"`
	Txn [][]any `json:"txn"`
}

func (s *server) handleTxn(msg maelstrom.Message) error {
	var body txnBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var replyTxn [][]any
	for _, t := range body.Txn {
		op := t[0].(string)
		key := int(t[1].(float64))
		var value int
		if t[2] != nil {
			value = int(t[2].(float64))
		}
		if op == "r" {
			if _, ok := s.kv[key]; !ok {
				replyTxn = append(replyTxn, []any{"r", key, nil})
				continue
			}
			replyTxn = append(replyTxn, []any{"r", key, s.kv[key]})
		}
		if op == "w" {
			s.kv[key] = value
			replyTxn = append(replyTxn, []any{"w", key, value})
		}
	}
	return s.n.Reply(msg, map[string]any{
		"type": "txn_ok",
		"txn":  replyTxn,
	})
}
