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

func TestRepairDeployStateFollowsTheMovedStagingRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldRoot := filepath.Join(home, "Games", "Amethyst")
	game := "Cyberpunk 2077"
	writeFile(t, filepath.Join(home, ".config", "AmethystModManager", "games", game, "deploy_state.json"), `{
  "deploy_active": true,
  "last_deployed": "default",
  "recovery_context": {
    "profile_dir": "`+oldRoot+`/`+game+`/profiles/default",
    "filemap_path": "`+oldRoot+`/`+game+`/filemap.txt",
    "staging_root": {"path": "`+oldRoot+`/`+game+`", "identity": {"device": 54, "inode": 1}},
    "game_root": {"path": "/games/Cyberpunk 2077"}
  }
}`)

	root := t.TempDir()
	migrateState(root, home, nil)

	newGameRoot := filepath.Join(stateEnv(root)["MOD_MANAGER_PROFILES_DIR"], game)
	journal := filepath.Join(stateEnv(root)["XDG_CONFIG_HOME"], "AmethystModManager", "games", game, "deploy_state.json")
	raw, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	context := data["recovery_context"].(map[string]any)
	if got := context["staging_root"].(map[string]any)["path"]; got != newGameRoot {
		t.Fatalf("staging_root = %v, want %s", got, newGameRoot)
	}
	if _, stale := context["staging_root"].(map[string]any)["identity"]; stale {
		t.Fatal("the moved folder kept the old identity")
	}
	if got := context["profile_dir"]; got != filepath.Join(newGameRoot, "profiles", "default") {
		t.Fatalf("profile_dir = %v", got)
	}
	if got := context["filemap_path"]; got != filepath.Join(newGameRoot, "filemap.txt") {
		t.Fatalf("filemap_path = %v", got)
	}
	if data["deploy_active"] != true {
		t.Fatal("the deployment was switched off")
	}
}
