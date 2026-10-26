package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherReportsFileAndDirChanges(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := make(chan []string, 4)
	w := New(20*time.Millisecond, func(changed []string) { events <- changed })
	w.Add(file, dir)
	w.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	select {
	case changed := <-events:
		t.Fatalf("unexpected event: %v", changed)
	case <-time.After(60 * time.Millisecond):
	}

	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(file, future, future); err != nil {
		t.Fatal(err)
	}
	select {
	case changed := <-events:
		if len(changed) != 1 || changed[0] != file {
			t.Fatalf("want exactly the file, got %v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("no event after file change")
	}

	if err := os.WriteFile(filepath.Join(dir, "new.json"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case changed := <-events:
		found := false
		for _, p := range changed {
			if p == dir {
				found = true
			}
		}
		if !found {
			t.Fatalf("dir change missing: %v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("no event after dir change")
	}

	if err := os.Remove(filepath.Join(dir, "new.json")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("no event after removal")
	}
}

func TestWatcherRunStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := New(10*time.Millisecond, nil)
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestWatcherCoalescesPerTick(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a")
	os.WriteFile(file, []byte("1"), 0o600)
	events := make(chan []string, 4)
	w := New(30*time.Millisecond, func(c []string) { events <- c })
	w.Add(file)
	w.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	os.WriteFile(file, []byte("2"), 0o600)
	os.WriteFile(file, []byte("3"), 0o600)
	select {
	case changed := <-events:
		if len(changed) != 1 {
			t.Fatalf("want one coalesced path, got %v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}
