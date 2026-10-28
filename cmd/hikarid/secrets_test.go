package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// keyringMock keeps the secret tests off the real desktop keyring.
func keyringMock(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(func() { keyring.MockInit() })
}

func TestSecretsAreWriteOnlyOverRPC(t *testing.T) {
	keyringMock(t)
	d, client := startDaemon(t)

	var before struct {
		Secrets []struct {
			Name      string `json:"name"`
			Set       bool   `json:"set"`
			Encrypted bool   `json:"encrypted"`
			Backend   string `json:"backend"`
		} `json:"secrets"`
		Encryption bool `json:"encryption"`
	}
	if err := client.Call("settings.secretStatus", map[string]any{}, &before); err != nil {
		t.Fatal(err)
	}
	if len(before.Secrets) != 1 || before.Secrets[0].Set {
		t.Fatalf("status = %+v", before)
	}
	if !before.Encryption {
		t.Fatal("a mocked keyring should be reported as encrypted storage")
	}

	// Seed the store directly: the RPC validates SteamGridDB keys over the
	// network, which a unit test must not do.
	if err := d.secrets.Set("steamgriddb", "abc123"); err != nil {
		t.Fatal(err)
	}
	d.artwork.SetKey("abc123")

	var after struct {
		Secrets []struct {
			Set       bool   `json:"set"`
			Encrypted bool   `json:"encrypted"`
			Backend   string `json:"backend"`
		} `json:"secrets"`
	}
	if err := client.Call("settings.secretStatus", map[string]any{}, &after); err != nil {
		t.Fatal(err)
	}
	if !after.Secrets[0].Set || after.Secrets[0].Backend != "keyring" {
		t.Fatalf("status after set = %+v", after.Secrets[0])
	}

	// The value must never travel back to a client.
	var raw json.RawMessage
	if err := client.Call("settings.secretStatus", map[string]any{}, &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "abc123") {
		t.Fatal("the secret value was returned to the client")
	}

	if err := client.Call("settings.setSecret", map[string]any{"name": "other", "value": "x"}, nil); err == nil {
		t.Fatal("an unknown secret name was accepted")
	}
	if err := client.Call("settings.setSecret", map[string]any{"name": "steamgriddb", "value": ""}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.secrets.Get("steamgriddb"); ok {
		t.Fatal("an empty value did not clear the secret")
	}
}

func TestPlaintextKeyIsMigratedOutOfSettings(t *testing.T) {
	keyringMock(t)
	cfgRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("HIKARI_HEROIC_CONFIG", filepath.Join(cfgRoot, "heroic-absent"))
	t.Setenv("HIKARI_SGDB_KEY", "")
	settingsPath := filepath.Join(cfgRoot, "hikari", "settings.toml")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[library]\nsteamgriddb_key = \"plain-key\"\n"
	if err := os.WriteFile(settingsPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := newDaemon(discardLogger(), true)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := d.secrets.Get("steamgriddb"); got != "plain-key" {
		t.Fatalf("keyring value = %q", got)
	}
	if d.artwork.APIKey != "plain-key" {
		t.Fatalf("the searcher did not get the key: %q", d.artwork.APIKey)
	}
	rewritten, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rewritten), "plain-key") {
		t.Fatal("settings.toml still contains the key")
	}
}
