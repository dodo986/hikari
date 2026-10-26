package watcher

import (
	"context"
	"os"
	"sort"
	"sync"
	"time"
)

type Watcher struct {
	interval time.Duration
	OnChange func(changed []string)

	mu      sync.Mutex
	targets []string
	last    map[string]time.Time
}

func New(interval time.Duration, onChange func(changed []string)) *Watcher {
	return &Watcher{
		interval: interval,
		OnChange: onChange,
		last:     map[string]time.Time{},
	}
}

func (w *Watcher) Add(paths ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.targets = append(w.targets, paths...)
}

func (w *Watcher) Snapshot() {
	w.mu.Lock()
	targets := append([]string{}, w.targets...)
	w.mu.Unlock()
	state := map[string]time.Time{}
	for _, p := range targets {
		state[p] = mtimeOf(p)
	}
	w.mu.Lock()
	w.last = state
	w.mu.Unlock()
}

func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if changed := w.poll(); len(changed) > 0 && w.OnChange != nil {
				w.OnChange(changed)
			}
		}
	}
}

func (w *Watcher) poll() []string {
	w.mu.Lock()
	targets := append([]string{}, w.targets...)
	w.mu.Unlock()

	var changed []string
	next := map[string]time.Time{}
	for _, p := range targets {
		m := mtimeOf(p)
		next[p] = m
		if prev, ok := w.last[p]; !ok || !m.Equal(prev) {
			changed = append(changed, p)
		}
	}
	w.mu.Lock()
	w.last = next
	w.mu.Unlock()
	sort.Strings(changed)
	return changed
}

func mtimeOf(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
