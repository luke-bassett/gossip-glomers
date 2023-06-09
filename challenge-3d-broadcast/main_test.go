package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func TestHashsetUnion(t *testing.T) {
	s1 := map[int]bool{0: true, 1: true}
	s2 := map[int]bool{0: true, 2: true, 3: true}
	got := hashsetUnion(s1, s2)
	want := map[int]bool{0: true, 1: true, 2: true, 3: true}
	if len(want) != len(got) {
		t.Errorf("Want len %v hashmap, got len %v", len(want), len(got))
	}
	for k := range want {
		if !got[k] {
			t.Errorf("Expected %v to be in %v", k, got)
		}
	}
}

func TestHashsetToSlice(t *testing.T) {
	hs := map[int]bool{0: true, 1: true}
	got := hashsetToSlice(hs)
	sort.Ints(got)
	want := []int{0, 1}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Want %v, got %v", want, got)
	}
}

func TestOthers(t *testing.T) {
	got := others([]string{"a", "b", "c"}, "b")
	want := []string{"a", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

func TestShareHandler(t *testing.T) {
	n := maelstrom.NewNode()
	s := &server{n: n}
	want := []int{0, 1, 2}
	body := map[string]interface{}{
		"type":     "share",
		"messages": want,
	}
	msgBodyJson, _ := json.Marshal(body)
	msg := maelstrom.Message{
		Body: msgBodyJson,
	}
	err := s.shareHandler(msg)
	if err != nil {
		t.Errorf("Got error %v from shareHandler", err)
	}
	sort.Ints(s.values)
	if !reflect.DeepEqual(s.values, want) {
		t.Errorf("Got %#v, want %#v", s.values, want)
	}

}
