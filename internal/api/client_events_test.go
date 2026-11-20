package api

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
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

// A job can publish its terminal event before the reply to the call that
// started it. That event must survive the Call and reach AwaitEvent, or a
// --follow command waits forever for progress that already happened.
func TestAwaitEventSeesEventBufferedDuringCall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stub.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		_ = ln.Close()
	})

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return
		}
		var req Request
		if json.Unmarshal([]byte(line), &req) != nil {
			return
		}
		event := `{"jsonrpc":"2.0","method":"install.progress","params":{"state":"done","percent":100}}` + "\n"
		reply, err := json.Marshal(&Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}})
		if err != nil {
			return
		}
		if _, err := conn.Write([]byte(event)); err != nil {
			return
		}
		if _, err := conn.Write(append(reply, '\n')); err != nil {
			return
		}
		<-release
	}()

	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	var out map[string]any
	if err := c.Call("game.install", nil, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}

	var mu sync.Mutex
	var seen []string
	c.OnEvent = func(method string, params json.RawMessage) {
		mu.Lock()
		seen = append(seen, method)
		mu.Unlock()
	}

	got := make(chan json.RawMessage, 1)
	failed := make(chan error, 1)
	go func() {
		params, err := c.AwaitEvent("install.progress", 30*time.Second)
		if err != nil {
			failed <- err
			return
		}
		got <- params
	}()

	select {
	case params := <-got:
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) != nil || p.State != "done" {
			t.Fatalf("AwaitEvent returned %s, want state done", params)
		}
	case err := <-failed:
		t.Fatalf("AwaitEvent: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("AwaitEvent read the socket instead of the event buffered during Call")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || seen[0] != "install.progress" {
		t.Fatalf("OnEvent saw %v, want [install.progress] exactly once", seen)
	}
}
