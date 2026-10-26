package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startServer(t *testing.T) (*Server, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hikari.sock")
	s := NewServer(slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx, path) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})
	waitForSocket(t, path)
	return s, path
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("socket never appeared")
}

func dial(t *testing.T, path string) *Client {
	t.Helper()
	c, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestEchoRoundTrip(t *testing.T) {
	s, path := startServer(t)
	s.Register("test.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"echo": string(params)}, nil
	})
	c := dial(t, path)
	var out map[string]string
	if err := c.Call("test.echo", map[string]any{"x": 1}, &out); err != nil {
		t.Fatal(err)
	}
	if out["echo"] == "" {
		t.Fatalf("echo result missing: %v", out)
	}
}

func TestMethodNotFound(t *testing.T) {
	_, path := startServer(t)
	c := dial(t, path)
	err := c.Call("nope", nil, nil)
	var re *Error
	if !errors.As(err, &re) || re.Code != CodeMethodNotFound {
		t.Fatalf("want -32601, got %v", err)
	}
}

func TestHandlerErrorBecomesRPCError(t *testing.T) {
	s, path := startServer(t)
	s.Register("test.fail", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, Errorf(CodeUnsupported, "game is imported, not managed")
	})
	s.Register("test.panic-ish", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, errors.New("plain failure")
	})
	c := dial(t, path)
	err := c.Call("test.fail", nil, nil)
	var re *Error
	if !errors.As(err, &re) || re.Code != CodeUnsupported || re.Message != "game is imported, not managed" {
		t.Fatalf("want code -32001 with message, got %v", err)
	}
	err = c.Call("test.panic-ish", nil, nil)
	if !errors.As(err, &re) || re.Code != CodeInternal {
		t.Fatalf("plain error should map to -32003, got %v", err)
	}
}

func TestEventsAfterSubscribeOnly(t *testing.T) {
	s, path := startServer(t)
	c := dial(t, path)

	s.Publish("install.progress", map[string]any{"percent": 1})

	if err := c.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	s.Publish("game.state", map[string]any{"state": "playing"})
	s.Publish("install.progress", map[string]any{"percent": 42.5})

	params, err := c.AwaitEvent("install.progress", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Percent float64 `json:"percent"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Percent != 42.5 {
		t.Fatalf("bad event params: %s (%v)", params, err)
	}
}

func TestSubscribeAllTopics(t *testing.T) {
	s, path := startServer(t)
	c := dial(t, path)
	if err := c.Subscribe(); err != nil {
		t.Fatal(err)
	}
	s.Publish("library.changed", map[string]any{"store": "gog"})
	params, err := c.AwaitEvent("library.changed", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) == 0 {
		t.Fatal("empty params")
	}
}

func TestStaleSocketFileIsReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hikari.sock")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewServer(slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx, path) }()
	t.Cleanup(cancel)
	waitForSocket(t, path)
	select {
	case err := <-done:
		t.Fatalf("server exited: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSecondServerRefuses(t *testing.T) {
	_, path := startServer(t)
	s2 := NewServer(slog.New(slog.DiscardHandler))
	err := s2.ListenAndServe(context.Background(), path)
	if err == nil {
		t.Fatal("second server should refuse to bind a live socket")
	}
}

func TestClientSurvivesEventInterleaving(t *testing.T) {
	s, path := startServer(t)
	s.Register("test.slow", func(ctx context.Context, params json.RawMessage) (any, error) {
		s.Publish("install.progress", map[string]any{"percent": 10})
		return map[string]any{"ok": true}, nil
	})
	c := dial(t, path)
	if err := c.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	var out map[string]bool
	if err := c.Call("test.slow", nil, &out); err != nil {
		t.Fatal(err)
	}
	if !out["ok"] {
		t.Fatalf("result lost among events: %v", out)
	}
}

func TestUnixSocketIsOwnerOnly(t *testing.T) {
	_, path := startServer(t)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("socket mode = %o, want 0700", fi.Mode().Perm())
	}
}

func TestSubscriberRemovedOnDisconnect(t *testing.T) {
	s, path := startServer(t)
	c := dial(t, path)
	if err := c.Subscribe(); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		n := len(s.subs)
		s.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("subscriber not removed after disconnect")
}

func TestReadLineStopsAtLimit(t *testing.T) {
	r := bufio.NewReader(bytes.NewReader(bytes.Repeat([]byte("a"), maxRequestLine+64)))
	if _, err := readLine(r, maxRequestLine); !errors.Is(err, errLineTooLong) {
		t.Fatalf("err = %v, want errLineTooLong", err)
	}
}

func TestOversizedRequestLineClosesTheConnection(t *testing.T) {
	_, path := startServer(t)
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	go func() {
		_, _ = conn.Write(bytes.Repeat([]byte("a"), maxRequestLine+64))
	}()
	conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	buf := make([]byte, 256)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("connection stayed open after an oversized request line")
	}
}
