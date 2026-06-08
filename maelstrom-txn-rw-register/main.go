package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n      *maelstrom.Node
	mu     sync.Mutex
	kv     map[int]versionedValue
	outbox map[string]map[txnID][][]any
}

type txnID struct {
	timestamp  int64
	originNode string
}

type versionedValue struct {
	txnID txnID
	value int
}

func newServer() *server {
	n := maelstrom.NewNode()
	kv := map[int]versionedValue{}
	outbox := map[string]map[txnID][][]any{}
	return &server{n: n, kv: kv, outbox: outbox}
}

func main() {
	s := newServer()

	s.n.Handle("txn", s.handleTxn)
	s.n.Handle("gossip", s.handleGossip)
	s.n.Handle("gossip_ok", s.handleGossipOk)

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

type txnBody struct {
	ID  int     `json:"msg_id"`
	Txn [][]any `json:"txn"`
}

func (s *server) handleTxn(msg maelstrom.Message) error {
	var body txnBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	id := txnID{timestamp: time.Now().UnixNano(), originNode: s.n.ID()}

	replyTxn := s.processTransaction(body.Txn, id)
	reply := map[string]any{"type": "txn_ok", "txn": replyTxn}
	s.queueGossip(body.Txn, id)

	return s.n.Reply(msg, reply)
}

func (s *server) queueGossip(t [][]any, id txnID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.n.NodeIDs() {
		if n == s.n.ID() {
			continue
		}
		if s.outbox[n] == nil {
			s.outbox[n] = map[txnID][][]any{}
		}
		s.outbox[n][id] = t
	}
}

func isNewer(check, against txnID) bool {
	if check.timestamp > against.timestamp {
		return true
	}
	if check.timestamp == against.timestamp && check.originNode > against.originNode {
		return true
	}
	return false
}

func (s *server) processTransaction(ops [][]any, id txnID) [][]any {
	var replyTxn [][]any

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, op := range ops {
		kind := op[0].(string)
		key := int(op[1].(float64))

		if kind == "r" {
			if _, ok := s.kv[key]; !ok {
				replyTxn = append(replyTxn, []any{"r", key, nil})
				continue
			}
			replyTxn = append(replyTxn, []any{"r", key, s.kv[key].value})
		}

		if kind == "w" {
			val := versionedValue{txnID: id, value: int(op[2].(float64))}
			replyTxn = append(replyTxn, []any{"w", key, val.value})

			stored, ok := s.kv[key]
			if !ok || stored.txnID == id || isNewer(id, stored.txnID) { // if new key, or same txn writing to the key again, or newer than existing value
				s.kv[key] = val
			}
		}
	}
	return replyTxn
}

func (s *server) gossip() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for neighbor, transactions := range s.outbox {
		for tId, transaction := range transactions {
			body := map[string]any{"type": "gossip", "txn": transaction, "id_ts": tId.timestamp, "id_node": tId.originNode}
			s.n.Send(neighbor, body)
		}
	}
}

type gossipBody struct {
	ID         int     `json:"msg_id"`
	Timestamp  int64   `json:"id_ts"`
	OriginNode string  `json:"id_node"`
	Txn        [][]any `json:"txn"`
}

func (s *server) handleGossip(msg maelstrom.Message) error {
	var body gossipBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	id := txnID{timestamp: body.Timestamp, originNode: body.OriginNode}

	s.processTransaction(body.Txn, id)
	reply := map[string]any{"type": "gossip_ok", "id_ts": body.Timestamp, "id_node": body.OriginNode}

	return s.n.Reply(msg, reply)
}

type gossipOkBody struct {
	ID         int    `json:"msg_id"`
	Timestamp  int64  `json:"id_ts"`
	OriginNode string `json:"id_node"`
}

func (s *server) handleGossipOk(msg maelstrom.Message) error {
	var body gossipOkBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	id := txnID{timestamp: body.Timestamp, originNode: body.OriginNode}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.outbox[msg.Src], id)
	return nil
}
