package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type RotatingWriter struct {
	Path string
	Max  int64

	mu   sync.Mutex
	f    *os.File
	size int64
}

func NewRotatingWriter(path string, max int64) (*RotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	w := &RotatingWriter{Path: path, Max: max}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotatingWriter) open() error {
	f, err := os.OpenFile(w.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f = f
	w.size = st.Size()
	return nil
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.Max > 0 && w.size+int64(len(p)) > w.Max {
		w.f.Close()
		w.f = nil
		_ = os.Rename(w.Path, w.Path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

type Ring struct {
	Max   int
	mu    sync.Mutex
	lines []string
	part  []byte
}

func NewRing(max int) *Ring { return &Ring{Max: max} }

func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.part = append(r.part, p...)
	for {
		i := bytes.IndexByte(r.part, '\n')
		if i < 0 {
			break
		}
		line := string(r.part[:i])
		r.part = r.part[i+1:]
		r.lines = append(r.lines, line)
		if len(r.lines) > r.Max {
			r.lines = r.lines[len(r.lines)-r.Max:]
		}
	}
	return len(p), nil
}

func (r *Ring) Lines(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.lines
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return slices.Clone(out)
}

func TailFile(path string, n int) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := splitLines(string(b))
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
