package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRingKeepsWholeLines(t *testing.T) {
	r := NewRing(3)
	r.Write([]byte("one\ntw"))
	r.Write([]byte("o\nthree\nfour\n"))
	got := r.Lines(0)
	want := []string{"two", "three", "four"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Lines = %v, want %v", got, want)
	}
	if tail := r.Lines(1); len(tail) != 1 || tail[0] != "four" {
		t.Fatalf("Lines(1) = %v", tail)
	}
}

func TestTailFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "log")
	os.WriteFile(p, []byte("a\nb\nc\n"), 0o644)
	lines, err := TailFile(p, 2)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, ",") != "b,c" {
		t.Fatalf("TailFile = %v", lines)
	}
}

func TestRotatingWriter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "log")
	w, err := NewRotatingWriter(p, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Write([]byte("first-line\n"))
	w.Write([]byte("second-line-that-overflows\n"))
	if _, err := os.Stat(p + ".1"); err != nil {
		t.Fatalf("expected a rotated file: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "second-line") {
		t.Fatalf("current log = %q", b)
	}
}
