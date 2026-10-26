package api

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestAwaitEventDispatchesToOnEvent(t *testing.T) {
	s, path := startServer(t)
	c := dial(t, path)
	if err := c.Subscribe(); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var seen []string
	c.OnEvent = func(method string, params json.RawMessage) {
		var p struct {
			Percent float64 `json:"percent"`
		}
		if json.Unmarshal(params, &p) != nil {
			return
		}
		mu.Lock()
		seen = append(seen, method)
		mu.Unlock()
	}

	s.Publish("game.state", map[string]any{"percent": 1.0})
	s.Publish("install.progress", map[string]any{"percent": 10.0})
	s.Publish("install.progress", map[string]any{"percent": 100.0})

	if _, err := c.AwaitEvent("install.progress", time.Second); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	first := append([]string(nil), seen...)
	mu.Unlock()
	if len(first) != 2 || first[0] != "game.state" || first[1] != "install.progress" {
		t.Fatalf("first wait forwarded %v, want [game.state install.progress]", first)
	}

	if _, err := c.AwaitEvent("install.progress", time.Second); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Fatalf("second wait forwarded %v, want three events", seen)
	}
}
