package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func mockKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInit() })
}

func TestSetAndGetUsesTheKeyring(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	if err := s.Set(SteamGridDB, "abc123"); err != nil {
		t.Fatal(err)
	}
	v, ok := s.Get(SteamGridDB)
	if !ok || v != "abc123" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	if _, err := os.Stat(s.path); err == nil {
		t.Fatal("the fallback file was written even though the keyring worked")
	}
	st := s.Status(SteamGridDB)
	if !st.Set || !st.Encrypted || st.Backend != "keyring" {
		t.Fatalf("status = %+v", st)
	}
}

func TestDeleteClearsEverything(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	if err := s.Set(SteamGridDB, "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(SteamGridDB); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(SteamGridDB); ok {
		t.Fatal("the secret survived a delete")
	}
	if st := s.Status(SteamGridDB); st.Set {
		t.Fatalf("status still reports a value: %+v", st)
	}
	if err := s.Delete(SteamGridDB); err != nil {
		t.Fatalf("deleting twice failed: %v", err)
	}
}

func TestDeleteSucceedsWhenNoKeyringIsRunning(t *testing.T) {
	keyring.MockInitWithError(errors.New("no keyring here"))
	t.Cleanup(keyring.MockInit)
	t.Setenv("HIKARI_SGDB_KEY", "")
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	if err := s.Set(SteamGridDB, "abc123"); err != nil {
		t.Fatalf("Set without a keyring failed: %v", err)
	}
	if v, ok := s.readFile()[SteamGridDB]; !ok || v != "abc123" {
		t.Fatalf("the key was not stored in the fallback file: %q %v", v, ok)
	}
	if err := s.Delete(SteamGridDB); err != nil {
		t.Fatalf("Delete without a keyring failed: %v", err)
	}
	if _, ok := s.readFile()[SteamGridDB]; ok {
		t.Fatal("the fallback file still holds the key")
	}
	if _, ok := s.Get(SteamGridDB); ok {
		t.Fatal("the secret survived a delete")
	}
	if err := s.Delete(SteamGridDB); err != nil {
		t.Fatalf("a second delete without a keyring failed: %v", err)
	}
}

func TestEnvironmentWins(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	if err := s.Set(SteamGridDB, "from-keyring"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HIKARI_SGDB_KEY", "from-env")
	v, ok := s.Get(SteamGridDB)
	if !ok || v != "from-env" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	if st := s.Status(SteamGridDB); st.Backend != "environment" {
		t.Fatalf("backend = %s", st.Backend)
	}
}

func TestUnknownNamesAreRejected(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	if err := s.Set("something-else", "v"); err == nil {
		t.Fatal("an unknown secret name was accepted")
	}
	if _, ok := s.Get("something-else"); ok {
		t.Fatal("an unknown secret name was read")
	}
}

func TestFallbackFileIsPrivate(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	// Simulate a machine without a keyring by writing through the file path.
	if err := s.writeFileKey(SteamGridDB, "abc123"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if st := s.Status(SteamGridDB); st.Encrypted {
		t.Fatal("the file backend claimed to be encrypted")
	}
}

func TestMigrateBlanksTopLevelValue(t *testing.T) {
	mockKeyring(t)
	s := New(filepath.Join(t.TempDir(), "secrets.json"))
	replacement, err := s.Migrate(SteamGridDB, "old-plaintext")
	if err != nil {
		t.Fatal(err)
	}
	if replacement != "" {
		t.Fatalf("replacement = %q, want empty", replacement)
	}
	v, ok := s.Get(SteamGridDB)
	if !ok || v != "old-plaintext" {
		t.Fatalf("value was not migrated: %q %v", v, ok)
	}
	if replacement, err := s.Migrate(SteamGridDB, "second"); err != nil || replacement != "" {
		t.Fatalf("second migrate = %q, %v", replacement, err)
	}
}
