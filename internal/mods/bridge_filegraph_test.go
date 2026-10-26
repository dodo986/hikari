package mods

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupAdoptsTheFilegraphExtension(t *testing.T) {
	root := t.TempDir()
	checkout := t.TempDir()
	native := filepath.Join(checkout, "native", "amethyst_filegraph", "target", "release")
	if err := os.MkdirAll(native, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(native, "amethyst_filegraph.so"), []byte("native"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := New(Options{Root: root})
	b.source = filepath.Join(checkout, "src")

	if err := b.adoptFilegraph(); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(root, "amethyst_filegraph.so")
	if content, err := os.ReadFile(copied); err != nil || string(content) != "native" {
		t.Fatalf("the extension was not copied next to the bridge: %v", err)
	}
	if b.snapshot().filegraph != copied {
		t.Fatalf("filegraph is %q, want %q", b.snapshot().filegraph, copied)
	}
}

func TestSetupSaysWhenTheFilegraphIsMissing(t *testing.T) {
	b := New(Options{Root: t.TempDir()})
	b.source = filepath.Join(t.TempDir(), "src")
	err := b.adoptFilegraph()
	if err == nil || !strings.Contains(err.Error(), "filegraph component was not found") {
		t.Fatalf("want an actionable sentence about the filegraph, got %v", err)
	}
}
