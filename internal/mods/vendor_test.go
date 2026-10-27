package mods

import (
	"os"
	"path/filepath"
	"testing"

	"hikari/internal/mods/backend"
)

func TestBackendIsUnpackedFromTheBinary(t *testing.T) {
	root := t.TempDir()
	bridge := New(Options{Root: root, Python: "/bin/sh"})
	if bridge.blocked != "" {
		t.Fatalf("bridge blocked with an embedded backend: %s", bridge.blocked)
	}
	src := filepath.Join(backendRoot(root), backend.Src)
	for _, name := range []string{"app_bootstrap.py", "Utils", "Nexus", "Games"} {
		if _, err := os.Stat(filepath.Join(src, name)); err != nil {
			t.Fatalf("%s missing from the unpacked backend: %v", name, err)
		}
	}
	if bridge.snapshot().source != src {
		t.Fatalf("bridge source = %q, want the unpacked tree %q", bridge.snapshot().source, src)
	}
}

func TestUnpackingDropsFilesAnOlderSnapshotLeft(t *testing.T) {
	root := t.TempDir()
	if err := materializeBackend(root); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(backendRoot(root), backend.Src, "Utils", "gone.py")
	if err := os.WriteFile(stale, []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An older snapshot is what makes the next start rewrite and prune.
	stamp := filepath.Join(backendRoot(root), "upstream.commit")
	if err := os.WriteFile(stamp, []byte("older\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backendRoot(root), backend.Src, "Utils", "deployment", "pipeline.py")
	if err := materializeBackend(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("a file from an older snapshot was left behind")
	}
	if content, err := os.ReadFile(marker); err != nil || len(content) == 0 {
		t.Fatalf("the rewrite lost %s: %v", marker, err)
	}
}
