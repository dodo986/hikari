package launch

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hikari/internal/engine"
)

func TestManagerPreparationFailureCrashesWithoutStarting(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "engine-ran")
	bin := filepath.Join(dir, "fake-engine")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ntouch "+marker+"\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager(slog.New(slog.DiscardHandler), engine.NewRunner(slog.New(slog.DiscardHandler)))

	states := make(chan string, 8)
	plan := &Plan{
		Argv: []string{bin},
		Prepare: func(ctx context.Context) error {
			return errors.New("wineboot failed")
		},
	}
	if err := m.Launch(context.Background(), "epic", "x", plan, func(state string, pid int) {
		states <- state
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case got := <-states:
			if got != StateCrashed {
				continue
			}
			if m.IsRunning("epic/x") {
				t.Fatal("failed preparation must clear the running entry")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("engine must not start when preparation fails (err=%v)", err)
			}
			return
		case <-deadline:
			t.Fatal("no crashed state after a failed preparation")
		}
	}
}
