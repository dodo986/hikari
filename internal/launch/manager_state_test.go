package launch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hikari/internal/engine"
)

func waitRunDone(t *testing.T, m *Manager, store, id string) RunLog {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if rec, ok := m.LastRun(store, id); ok && !rec.Running {
			return rec
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the launch never finished")
	return RunLog{}
}

// Wine and Proton write warnings to stderr on almost every run, so a clean exit
// has to stay a clean exit: the first stderr line is only the error message
// when the run actually failed (non-zero exit).
func TestCleanExitWithStderrIsNotACrash(t *testing.T) {
	m := NewManager(slog.New(slog.DiscardHandler), engine.NewRunner(slog.New(slog.DiscardHandler)))
	plan := &Plan{Argv: []string{"/bin/sh", "-c", "echo 'fixme: harmless warning' >&2; exit 0"}}
	if err := m.Launch(context.Background(), "local", "clean", plan, nil); err != nil {
		t.Fatal(err)
	}
	rec := waitRunDone(t, m, "local", "clean")
	if rec.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", rec.ExitCode)
	}
	if rec.State != StateStopped {
		t.Fatalf("state = %q, want %q (error %q)", rec.State, StateStopped, rec.Error)
	}
	if rec.Error != "" {
		t.Fatalf("error = %q, want empty on a clean exit", rec.Error)
	}
}

// A stop that lands while the prefix is being prepared must not start the game
// and then tell the UI it is playing.
func TestStopDuringPrepareDoesNotStartTheGame(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	prepared := make(chan struct{})
	m := NewManager(slog.New(slog.DiscardHandler), engine.NewRunner(slog.New(slog.DiscardHandler)))
	plan := &Plan{
		Argv: []string{"/bin/sh", "-c", "echo started > " + marker},
		Prepare: func(ctx context.Context) error {
			close(prepared)
			<-ctx.Done()
			return nil
		},
	}
	states := make(chan string, 16)
	if err := m.Launch(context.Background(), "local", "late", plan, func(state string, pid int) {
		states <- state
	}); err != nil {
		t.Fatal(err)
	}
	<-prepared
	if err := m.Stop("local/late"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	close(states)
	for state := range states {
		if state == StatePlaying {
			t.Fatal("the game was started after the stop request")
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the game process ran after the stop request")
	}
	rec, ok := m.LastRun("local", "late")
	if !ok || rec.State != StateStopped || rec.Running {
		t.Fatalf("last run = %+v, want a stopped session", rec)
	}
}
