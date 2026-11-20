package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hikari/internal/api"
	"hikari/internal/config"
)

func readEndpointFile(t *testing.T) Endpoint {
	t.Helper()
	b, err := os.ReadFile(config.WSEndpointPath())
	if err != nil {
		t.Fatalf("endpoint file missing: %v", err)
	}
	var ep Endpoint
	if err := json.Unmarshal(b, &ep); err != nil {
		t.Fatalf("endpoint file unreadable: %v", err)
	}
	return ep
}

func waitEndpoint(t *testing.T) Endpoint {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(config.WSEndpointPath()); err == nil {
			var ep Endpoint
			if json.Unmarshal(b, &ep) == nil && ep.URL != "" {
				return ep
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the first daemon never published its endpoint")
	return Endpoint{}
}

// Regression: a second daemon start used to publish its own ws.json (fresh
// port and token) BEFORE checking the socket, then exit when the socket turned
// out to be owned. That stranded every window of the running daemon: the
// endpoint file pointed at a listener that never existed. A losing start must
// fail without touching the running daemon's endpoint.
func TestSecondStartKeepsTheRunningEndpoint(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("HIKARI_HEROIC_CONFIG", filepath.Join(t.TempDir(), "heroic-absent"))
	socket := filepath.Join(runtime, "hikari.sock")
	log := discardLogger()

	d1, err := newDaemon(log, true)
	if err != nil {
		t.Fatal(err)
	}
	srv1 := api.NewServer(log)
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	done1 := make(chan error, 1)
	go func() { done1 <- serve(ctx1, log, d1, srv1, socket, false) }()
	first := waitEndpoint(t)

	d2, err := newDaemon(log, true)
	if err != nil {
		t.Fatal(err)
	}
	srv2 := api.NewServer(log)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := serve(ctx2, log, d2, srv2, socket, false); err == nil {
		t.Fatal("a second daemon must not start while the socket is owned")
	}

	after, err := os.ReadFile(config.WSEndpointPath())
	if err != nil {
		t.Fatalf("the losing start deleted the running daemon's endpoint: %v", err)
	}
	var got Endpoint
	if err := json.Unmarshal(after, &got); err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("the losing start rewrote the endpoint:\n before %+v\n after  %+v", first, got)
	}

	cancel1()
	if err := <-done1; err != nil {
		t.Fatalf("first daemon stopped with %v", err)
	}
}

// The shutdown cleanup must only remove the endpoint file while it still
// describes this process, so a late or losing shutdown cannot erase the
// endpoint of the daemon that actually owns the socket.
func TestRemoveEndpointKeepsAnotherDaemonsFile(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	path := config.WSEndpointPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := fmt.Sprintf(`{"url":"ws://127.0.0.1:9/rpc","token":"x","pid":%d}`, os.Getpid()+1000000)
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	removeEndpoint()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("removeEndpoint deleted another daemon's endpoint file")
	}

	own := fmt.Sprintf(`{"url":"ws://127.0.0.1:9/rpc","token":"x","pid":%d}`, os.Getpid())
	if err := os.WriteFile(path, []byte(own), 0o600); err != nil {
		t.Fatal(err)
	}
	removeEndpoint()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("removeEndpoint should remove this process's own endpoint")
	}
}
