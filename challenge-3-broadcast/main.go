package main 

import (
    "encoding/json"
    "log"

    maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
    n := maelstrom.NewNode()
    s := &server{n: n}

    n.Handle("broadcast", s.broadcastHandler)
    n.Handle("read", s.readHandler)
    n.Handle("topology", s.topologyHandler)

    if err := n.Run(); err != nil {
        log.Fatal(err)
    }

}

type server struct {
    n *maelstrom.Node
    messages []int
}

func (s *server) broadcastHandler(msg maelstrom.Message) error {
    var body map[string]any
    if err := json.Unmarshal(msg.Body, &body); err != nil {
        return err
    }
    s.messages = append(s.messages, int(body["message"].(float64)))
    return s.n.Reply(msg, map[string]any{
        "type": "broadcast_ok",
    })
}

func (s *server) readHandler(msg maelstrom.Message) error {
    var body map[string]any
    if err := json.Unmarshal(msg.Body, &body); err != nil {
        return err
    }
    body["type"] = "read_ok"
    body["messages"] = s.messages
    return s.n.Reply(msg, body)
}

func (s *server) topologyHandler(msg maelstrom.Message) error {
    var body map[string]any
    if err := json.Unmarshal(msg.Body, &body); err != nil {
        return err
    }
    delete(body, "topology")
    body["type"] = "topology_ok"

    return s.n.Reply(msg, body)
}

