package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n       *maelstrom.Node
	kv      *maelstrom.KV
	offsets map[string]int
}

func newServer() *server {
	n := maelstrom.NewNode()
	kv := maelstrom.NewLinKV(n)
	return &server{
		n:  n,
		kv: kv,
	}
}

func main() {
	s := newServer()

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
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var offset int
	nextOffset, err := s.readNextOffset(ctx, "next/"+body.Key)
	if err != nil {
		return err
	}
	offset = nextOffset
	for {

		msgKey := fmt.Sprintf("log/%s/%d", body.Key, offset)
		// from will only be nil if the key doesn't exist yet
		err = s.kv.CompareAndSwap(ctx, msgKey, nil, body.Msg, true)
		if err == nil {
			break
		}
		if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.PreconditionFailed {
			// getting explosions of messages as nodes fight over next offset,
			// this helps with that. Other approaches could be reserving ranges
			// of offsets, or serializing per key and node. I'm just doing the
			// simple one-line approach for now to keep it simple.
			// time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			offset++
			continue
		}
		return err
	}

	err = s.kv.Write(ctx, "next/"+body.Key, offset+1)
	if err != nil {
		return err
	}
	return s.n.Reply(msg, map[string]any{"type": "send_ok", "offset": offset})
}

func (s *server) readNextOffset(ctx context.Context, key string) (int, error) {
	nextOffset, err := s.kv.Read(ctx, key)
	if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.KeyDoesNotExist {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return nextOffset.(int), nil
}

func (s *server) readValue(ctx context.Context, key string, offset int) (int, bool, error) {
	k := fmt.Sprintf("log/%s/%d", key, offset)
	value, err := s.kv.Read(ctx, k)
	if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.KeyDoesNotExist {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return value.(int), true, nil
}

type pollBody struct {
	Offsets map[string]int `json:"offsets"`
}

func (s *server) handlePoll(msg maelstrom.Message) error {
	var body pollBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	// the challenge says "Your server can choose to return as many messages for
	// each log as it chooses", I'll do 3 I guess (is zero OK?)
	msgs := map[string][][]int{}
	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	const maxMessages = 20
	for key, offset := range body.Offsets {
		next, err := s.readNextOffset(ctx, "next/"+key)
		if err != nil {
			return err
		}
		count := min(maxMessages, next-offset)
		if count <= 0 {
			continue
		}
		var messagesFromOffset [][]int
		for i := 0; i < count; i++ {
			value, ok, err := s.readValue(ctx, key, offset+i)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			messagesFromOffset = append(messagesFromOffset, []int{offset + i, value})
		}
		msgs[key] = messagesFromOffset
	}
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

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for key, offset := range body.Offsets {
		// we don't need to do a CAS loop here, in the case that we process the
		// farthest forward offset and then it gets clobbered by an older offset
		// all that may happen is a re-read of some messages. So we maintain
		// at-least-once semantics, and keep this light/quick.
		// optimization area: write to a seqkv for the committed offsets
		err := s.kv.Write(ctx, "offset/"+key, offset)
		if err != nil {
			return err
		}
	}
	return s.n.Reply(msg, map[string]any{"type": "commit_offsets_ok"})
}

type listCommittedOffsetBody struct {
	Keys []string `json:"keys"`
}

func (s *server) readOffset(ctx context.Context, key string) (int, bool, error) {
	offset, err := s.kv.Read(ctx, key)
	if rpcErr, ok := err.(*maelstrom.RPCError); ok && rpcErr.Code == maelstrom.KeyDoesNotExist {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}
	o := offset.(int)
	return o, true, nil
}

func (s *server) handleListCommittedOffsets(msg maelstrom.Message) error {
	var body listCommittedOffsetBody
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	committedOffsets := map[string]int{}
	for _, key := range body.Keys {

		offset, ok, err := s.readOffset(ctx, "offset/"+key)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		committedOffsets[key] = offset
	}
	return s.n.Reply(msg, map[string]any{"type": "list_committed_offsets_ok", "offsets": committedOffsets})
}
