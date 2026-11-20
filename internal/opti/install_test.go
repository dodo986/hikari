package opti

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReinstallBacksUpGameUpdatedHook reproduces the real data loss: the game
// rewrites its hook DLL after Hikari's first install, and the second install
// used to keep the stale first backup. Uninstall then restored the pre-update
// DLL and the game's own update was gone.
func TestReinstallBacksUpGameUpdatedHook(t *testing.T) {
	versionDir := fakeReleaseDir(t)
	gameDir := t.TempDir()
	hook := filepath.Join(gameDir, "dxgi.dll")
	bak := filepath.Join(gameDir, BackupDir, "dxgi.dll")

	if err := os.WriteFile(hook, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(versionDir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "payload")
	assertFileContent(t, bak, "original")

	// the game updates its own hook DLL after Hikari installed
	if err := os.WriteFile(hook, []byte("game-update"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(versionDir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "payload")
	assertFileContent(t, bak, "game-update")

	if err := RemoveFromGameDir(gameDir); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "game-update")
}

// TestReinstallKeepsOriginalBackupWhenHookIsPayload is the other half of the
// rule: when nothing touched our payload, the first backup (the game's own
// DLL) must survive a reinstall untouched.
func TestReinstallKeepsOriginalBackupWhenHookIsPayload(t *testing.T) {
	versionDir := fakeReleaseDir(t)
	gameDir := t.TempDir()
	hook := filepath.Join(gameDir, "dxgi.dll")
	bak := filepath.Join(gameDir, BackupDir, "dxgi.dll")

	if err := os.WriteFile(hook, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(versionDir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(versionDir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "payload")
	assertFileContent(t, bak, "original")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(b) != want {
		t.Fatalf("%s = %q, want %q", path, b, want)
	}
}

// TestReinstallAfterHikariUpgradeKeepsTheGamesBackup: upgrading Hikari replaces
// the release payload, so the hook on disk (the previous payload) differs from
// the incoming one. It is still our file, not the game's, and the manifest hash
// has to say so; otherwise the upgrade overwrites the game's own backup with
// the old payload and uninstall restores that instead.
func TestReinstallAfterHikariUpgradeKeepsTheGamesBackup(t *testing.T) {
	gameDir := t.TempDir()
	hook := filepath.Join(gameDir, "dxgi.dll")
	bak := filepath.Join(gameDir, BackupDir, "dxgi.dll")

	v1 := t.TempDir()
	if err := os.WriteFile(filepath.Join(v1, "OptiScaler.dll"), []byte("payload-v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	v2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(v2, "OptiScaler.dll"), []byte("payload-v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(hook, []byte("game-original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(v1, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, bak, "game-original")

	if err := CopyVersionTree(v2, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "payload-v2")
	assertFileContent(t, bak, "game-original")

	if err := RemoveFromGameDir(gameDir); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, hook, "game-original")
}

// The manifest sits in the game folder, so it is not trustworthy input. A
// damaged or hand-edited injection_dll must never be joined onto the game
// directory: an empty value made hook == gameDir, and a traversal could rename
// or delete a file outside the install.
func TestRemoveFromGameDirRejectsBadManifestHook(t *testing.T) {
	for _, bad := range []string{"", "..", "../../victim.dll", "not-a-hook.exe"} {
		gameDir := t.TempDir()
		body := `{"version":"v1","injection_dll":"` + bad + `"}`
		if err := os.WriteFile(filepath.Join(gameDir, ManifestFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		victim := filepath.Join(filepath.Dir(gameDir), "victim.dll")
		if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := RemoveFromGameDir(gameDir); err == nil {
			t.Fatalf("RemoveFromGameDir accepted injection_dll %q", bad)
		}
		if _, err := os.Stat(victim); err != nil {
			t.Fatalf("injection_dll %q removed a file outside the game dir: %v", bad, err)
		}
	}
}
