package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/library"
)

// A typo in the executable is invisible until the game refuses to start, so
// saving one has to refuse it, and clearing the field has to stay allowed.
func TestSettingsCheckTheChosenExecutable(t *testing.T) {
	_, client := startDaemon(t)
	root := t.TempDir()
	exe := filepath.Join(root, "bin", "x64", "game.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("MZ"), 0o755); err != nil {
		t.Fatal(err)
	}

	var game library.Game
	if err := client.Call("game.add_local", map[string]any{
		"title": "Rip", "executable": exe, "platform": "Windows",
	}, &game); err != nil {
		t.Fatal(err)
	}
	set := func(target string) error {
		return client.Call("game.settings.set", map[string]any{
			"store": "local", "id": game.ID,
			"settings": map[string]any{"target_exe": target},
		}, nil)
	}

	// A local entry's install path is the executable's folder, which is what a
	// relative target is resolved against at launch too.
	if err := set("game.exe"); err != nil {
		t.Fatalf("a relative executable the game does have should save: %v", err)
	}
	err := set(filepath.Join(root, "bin", "x64", "missing.exe"))
	if err == nil {
		t.Fatal("a missing executable was accepted")
	}
	if !strings.Contains(err.Error(), "no executable at") {
		t.Fatalf("error = %v", err)
	}
	if err := set(""); err != nil {
		t.Fatalf("clearing the field must always work: %v", err)
	}
}
