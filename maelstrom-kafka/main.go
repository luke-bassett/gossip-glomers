package main

import (
	"encoding/json"
	"log"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n      *maelstrom.Node
	mu     sync.Mutex
	topics map[string]*topic
}

type topic struct {
	events          []int
	committedOffset int
}

func main() {
	s := &server{
		n:      maelstrom.NewNode(),
		topics: map[string]*topic{},
	}

	s.n.Handle("send", s.handleSend)
	s.n.Handle("poll", s.handlePoll)
	s.n.Handle("commit_offsets", s.handleCommitOffset)
	s.n.Handle("list_committed_offsets", s.handleListCommittedOffsets)

	if err := s.n.Run(); err != nil {
		log.Fatal(err)
	}
}

type sendBody struct {
	Key string `json:"key"`
	Msg int    `json:"msg"`
}

func (s *server) handleSend(msg maelstrom.Message) error {
	var body sendBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	s.mu.Lock()
	if _, ok := s.topics[body.Key]; !ok {
		s.topics[body.Key] = &topic{}
	}
	s.topics[body.Key].events = append(s.topics[body.Key].events, body.Msg)
	offset := len(s.topics[body.Key].events) - 1
	s.mu.Unlock()

	return s.n.Reply(msg, map[string]any{"type": "send_ok", "offset": offset})
}

type pollBody struct {
	Offsets map[string]int `json:"offsets"`
}

func (s *server) handlePoll(msg maelstrom.Message) error {
	var body pollBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	// the challenge says "Your server can choose to return as many messages for each log as it chooses", I'll do 3 I guess (is zero OK?)
	msgs := map[string][][]int{}
	s.mu.Lock()
	for key, offset := range body.Offsets {
		var messagesFromOffset [][]int
		if _, ok := s.topics[key]; !ok {
			continue
		}
		for i := range 3 {
			if offset+i >= len(s.topics[key].events) {
				break
			}
			messagesFromOffset = append(messagesFromOffset, []int{offset + i, s.topics[key].events[offset+i]})
		}
		msgs[key] = messagesFromOffset
	}
	s.mu.Unlock()
	return s.n.Reply(msg, map[string]any{"type": "poll_ok", "msgs": msgs})
}

type commitOffsetBody struct {
	Offsets map[string]int `json:"offsets"`
}

func (s *server) handleCommitOffset(msg maelstrom.Message) error {
	var body commitOffsetBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	for key, offset := range body.Offsets {
		s.topics[key].committedOffset = offset
	}
	s.mu.Unlock()
	return s.n.Reply(msg, map[string]any{"type": "commit_offsets_ok"})
}

type listCommittedOffsetBody struct {
	Keys []string `json:"keys"`
}

func (s *server) handleListCommittedOffsets(msg maelstrom.Message) error {
	var body listCommittedOffsetBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	committedOffsets := map[string]int{}
	s.mu.Lock()
	for _, key := range body.Keys {
		if _, ok := s.topics[key]; !ok {
			continue
		}
		committedOffsets[key] = s.topics[key].committedOffset
	}
	s.mu.Unlock()
	return s.n.Reply(msg, map[string]any{"type": "list_committed_offsets_ok", "offsets": committedOffsets})
}
