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
	// The watched directory fires too now that its contents are part of its
	// fingerprint, and its snapshot can be read just before or just after the
	// file's, so collect events until the file target itself shows up.
	if !waitForPath(events, file) {
		t.Fatal("no event reported the changed file")
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

// TestWatcherSeesInPlaceRewriteInDirectory rewrites an existing file inside a
// watched directory without changing the directory's entry set, which leaves
// the directory's own mtime untouched. The snapshot must still notice it.
func TestWatcherSeesInPlaceRewriteInDirectory(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store_cache")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(store, "library.json")
	if err := os.WriteFile(file, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	events := make(chan []string, 4)
	w := New(10*time.Millisecond, func(c []string) { events <- c })
	w.Add(store)
	w.Snapshot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	select {
	case changed := <-events:
		t.Fatalf("unexpected event: %v", changed)
	case <-time.After(50 * time.Millisecond):
	}

	if err := os.WriteFile(file, []byte(`{"a":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(file, future, future); err != nil {
		t.Fatal(err)
	}
	select {
	case changed := <-events:
		if len(changed) != 1 || changed[0] != store {
			t.Fatalf("want the watched directory, got %v", changed)
		}
	case <-time.After(time.Second):
		t.Fatal("in-place rewrite inside the watched directory did not fire")
	}
}

func TestWatcherNonPositiveIntervalFallsBack(t *testing.T) {
	w := New(0, nil)
	if w.interval != DefaultInterval {
		t.Fatalf("interval = %v, want %v", w.interval, DefaultInterval)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}

func hasPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// waitForPath drains change events until one reports want, or a second passes.
func waitForPath(events <-chan []string, want string) bool {
	deadline := time.After(time.Second)
	for {
		select {
		case changed := <-events:
			if hasPath(changed, want) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
