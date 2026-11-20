package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultInterval is used when New is given a non-positive interval.
	DefaultInterval = 3 * time.Second
	// watchDepth bounds how deep a directory snapshot walks.
	watchDepth = 3
	// watchEntries bounds how many entries one snapshot records.
	watchEntries = 8192
)

type Watcher struct {
	interval time.Duration
	OnChange func(changed []string)

	mu      sync.Mutex
	targets []string
	last    map[string]string
}

func New(interval time.Duration, onChange func(changed []string)) *Watcher {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Watcher{
		interval: interval,
		OnChange: onChange,
		last:     map[string]string{},
	}
}

func (w *Watcher) Add(paths ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.targets = append(w.targets, paths...)
}

func (w *Watcher) Snapshot() {
	w.mu.Lock()
	defer w.mu.Unlock()
	state := make(map[string]string, len(w.targets))
	for _, p := range w.targets {
		state[p] = fingerprint(p)
	}
	w.last = state
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
	prev := w.last
	w.mu.Unlock()

	var changed []string
	next := make(map[string]string, len(targets))
	for _, p := range targets {
		sig := fingerprint(p)
		next[p] = sig
		if old, ok := prev[p]; !ok || sig != old {
			changed = append(changed, p)
		}
	}
	w.mu.Lock()
	w.last = next
	w.mu.Unlock()
	sort.Strings(changed)
	return changed
}

// fingerprint describes a watched path well enough to notice an in-place
// rewrite: a single file is its mtime, while a directory is the sorted set of
// its entries and their mtimes, walked to a bounded depth and size. A plain
// directory mtime only moves when an entry is added or removed, which is why
// rewriting a file inside one used to go unnoticed.
func fingerprint(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if !fi.IsDir() {
		return strconv.FormatInt(fi.ModTime().UnixNano(), 10)
	}
	var b strings.Builder
	remaining := watchEntries
	walkState(&b, path, "", 0, &remaining)
	return b.String()
}

func walkState(b *strings.Builder, root, rel string, depth int, remaining *int) {
	if *remaining <= 0 || depth > watchDepth {
		return
	}
	entries, err := os.ReadDir(filepath.Join(root, rel))
	if err != nil {
		return
	}
	for _, e := range entries {
		if *remaining <= 0 {
			return
		}
		*remaining--
		name := filepath.Join(rel, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		b.WriteString(name)
		b.WriteByte('\t')
		b.WriteString(strconv.FormatInt(info.ModTime().UnixNano(), 10))
		b.WriteByte('\n')
		if e.IsDir() {
			walkState(b, root, name, depth+1, remaining)
		}
	}
}
