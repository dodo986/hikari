package mods

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateStateMovesConfigAndRepointsStaging(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeFile(t, filepath.Join(home, ".config", "AmethystModManager", "games", "Skyrim", "paths.json"),
		`{"game_path":"/games/Skyrim","staging_path":"`+home+`/Games/Amethyst/Skyrim","deploy_mode":"hardlink"}`)
	writeFile(t, filepath.Join(home, "Games", "Amethyst", "Skyrim", "profiles", "default", "modlist.txt"), "SkyUI\n")

	root := t.TempDir()
	migrateState(root, home, nil)

	env := stateEnv(root)
	staging := filepath.Join(env["MOD_MANAGER_PROFILES_DIR"], "Skyrim")
	if _, err := os.Stat(filepath.Join(staging, "profiles", "default", "modlist.txt")); err != nil {
		t.Fatalf("staged mods were not carried over: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(env["XDG_CONFIG_HOME"], "AmethystModManager", "games", "Skyrim", "paths.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data["staging_path"] != staging {
		t.Fatalf("staging_path = %v, want %s", data["staging_path"], staging)
	}
	if data["game_path"] != "/games/Skyrim" {
		t.Fatalf("game_path was rewritten: %v", data["game_path"])
	}
}

func TestMigrateStateLeavesTheOriginalInPlace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	original := filepath.Join(home, "Games", "Amethyst", "Skyrim", "mods", "SkyUI.esp")
	writeFile(t, original, "plugin")

	root := t.TempDir()
	migrateState(root, home, nil)
	migrateState(root, home, nil)

	if content, err := os.ReadFile(original); err != nil || string(content) != "plugin" {
		t.Fatalf("the Amethyst copy was disturbed: %v", err)
	}
	staging := filepath.Join(stateEnv(root)["MOD_MANAGER_PROFILES_DIR"], "Skyrim", "mods", "SkyUI.esp")
	if content, err := os.ReadFile(staging); err != nil || string(content) != "plugin" {
		t.Fatalf("the migrated copy is wrong: %v", err)
	}
}

func TestStateEnvKeepsTheBackendOutOfAmethystFolders(t *testing.T) {
	root := t.TempDir()
	env := stateEnv(root)
	for name, path := range env {
		if !underDir(path, root) {
			t.Fatalf("%s = %q, want it under %q", name, path, root)
		}
	}
	if env["XDG_CONFIG_HOME"] == "" || env["MOD_MANAGER_PROFILES_DIR"] == "" {
		t.Fatalf("stateEnv left a path unset: %v", env)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
