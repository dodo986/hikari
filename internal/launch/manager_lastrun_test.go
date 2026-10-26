package launch

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"hikari/internal/engine"
)

func TestLastRunKeepsOutputAndError(t *testing.T) {
	run := engine.NewRunner(slog.New(slog.DiscardHandler))
	m := NewManager(slog.New(slog.DiscardHandler), run)

	plan := &Plan{Argv: []string{"/bin/sh", "-c", "echo boom >&2; exit 3"}}
	var states []string
	err := m.Launch(context.Background(), "local", "x", plan, func(state string, pid int) {
		states = append(states, state)
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rec, ok := m.LastRun("local", "x")
		if ok && !rec.Running {
			if rec.State != StateCrashed {
				t.Fatalf("state = %q, want %q (states: %v)", rec.State, StateCrashed, states)
			}
			if !strings.Contains(rec.Error, "boom") {
				t.Errorf("error = %q, want the engine's message", rec.Error)
			}
			if rec.ExitCode != 3 {
				t.Errorf("exit code = %d, want 3", rec.ExitCode)
			}
			if len(rec.Lines) == 0 || !strings.Contains(strings.Join(rec.Lines, "\n"), "boom") {
				t.Errorf("lines = %v, want the stderr line", rec.Lines)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("launch never finished")
}
