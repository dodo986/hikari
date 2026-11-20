package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The socket bind is the ownership claim: a second daemon must fail at Listen,
// before it can publish an endpoint or serve anything.
func TestListenClaimsTheSocketBeforeServing(t *testing.T) {
	s := NewServer(slog.New(slog.DiscardHandler))
	s.Register("test.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	path := filepath.Join(t.TempDir(), "hikari.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := s.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Listen(path); err == nil {
		t.Fatal("a second Listen must fail while the socket is owned")
	}

	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	waitForSocket(t, path)

	c := dial(t, path)
	var out map[string]bool
	if err := c.Call("test.echo", nil, &out); err != nil || !out["ok"] {
		t.Fatalf("serve on the claimed listener: %v %v", out, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not stop")
	}
}

// A stale socket left by a dead daemon must not block a fresh claim.
func TestListenReplacesAStaleSocket(t *testing.T) {
	s := NewServer(slog.New(slog.DiscardHandler))
	path := filepath.Join(t.TempDir(), "hikari.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first, err := s.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a daemon that died without cleanup: close the listener but
	// leave the socket file behind.
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	ln, err := s.Listen(path)
	if err != nil {
		t.Fatalf("a stale socket must be replaceable: %v", err)
	}
	go func() { _ = s.Serve(ctx, ln) }()
	waitForSocket(t, path)
}

// makeStaleSocket leaves a real socket file behind with nothing listening, the
// way a daemon that died without cleanup does.
func makeStaleSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stale socket was not left behind: %v", err)
	}
}

// Regression: starting several daemons at once against a stale socket used to
// let more than one return a listener. Liveness probe, remove and bind were not
// atomic, so two starts could both remove the dead file and bind, the second
// Remove unlinking the winner's fresh socket. The exclusive flock on
// path+".lock" makes exactly one start win.
func TestConcurrentListenClaimsExactlyOne(t *testing.T) {
	dir := t.TempDir()
	const racers = 16

	for round := 0; round < 6; round++ {
		path := filepath.Join(dir, fmt.Sprintf("hikari-%d.sock", round))
		makeStaleSocket(t, path)

		var released atomic.Bool
		lns := make([]net.Listener, racers)
		errs := make([]error, racers)
		var started, wg sync.WaitGroup
		started.Add(racers)
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				s := NewServer(slog.New(slog.DiscardHandler))
				started.Done()
				// Spin (not park) so every racer is already on a P and enters
				// the probe/remove/bind window the instant it is released.
				for !released.Load() {
					runtime.Gosched()
				}
				lns[i], errs[i] = s.Listen(path)
			}(i)
		}
		started.Wait()
		released.Store(true)
		wg.Wait()

		var winners []net.Listener
		for i := 0; i < racers; i++ {
			if errs[i] == nil {
				winners = append(winners, lns[i])
			}
		}
		if len(winners) != 1 {
			t.Fatalf("round %d: %d of %d concurrent Listen calls succeeded, want exactly 1", round, len(winners), racers)
		}
		if err := winners[0].Close(); err != nil {
			t.Fatalf("round %d: closing the winner: %v", round, err)
		}
	}
}
