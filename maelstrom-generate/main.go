package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	var i atomic.Int64

	n.Handle("generate", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		id := fmt.Sprintf("%s-%d", n.ID(), i.Add(1))
		// alt approach: id := uuid.New().String()
		body["type"] = "generate_ok"
		body["id"] = id

		return n.Reply(msg, body)
	})
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}
