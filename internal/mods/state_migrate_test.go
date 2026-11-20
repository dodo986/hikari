package mods

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMigrateStateRetriesAfterAPartialCopy guards against a half-migrated tree
// being mistaken for a finished one: when the walk fails, copyTree drops the
// partial destination so the next start copies the rest.
func TestMigrateStateRetriesAfterAPartialCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a 000 directory, so the walk cannot fail")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldStaging := filepath.Join(home, "Games", "Amethyst")
	blocked := filepath.Join(oldStaging, "aaa-blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	// Sorts after the blocked directory, so the failing walk never reaches it.
	writeFile(t, filepath.Join(oldStaging, "zzz-late", "late.esp"), "late")

	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("permissions are not enforced here, so the walk cannot fail")
	}

	root := t.TempDir()
	New(Options{Root: root, Home: home, Python: "/bin/sh"})

	dest := filepath.Join(root, "games")
	late := filepath.Join(dest, "zzz-late", "late.esp")
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("the failed migration left a partial destination behind")
	}

	if err := os.Chmod(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	New(Options{Root: root, Home: home, Python: "/bin/sh"})
	if _, err := os.Stat(late); err != nil {
		t.Fatalf("the retried migration did not complete: %v", err)
	}
}
