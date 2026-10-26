package launch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/config"
	"hikari/internal/engine"
)

func fakeWineboot(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "wine-fake")
	script := "#!/bin/sh\necho \"$@\" > " + filepath.Join(dir, "argv.txt") + "\nmkdir -p \"$WINEPREFIX\"\ntouch \"$WINEPREFIX/system.reg\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestEnsurePrefixInitialisesPlainWine(t *testing.T) {
	dir := t.TempDir()
	bin := fakeWineboot(t, dir)
	prefix := filepath.Join(dir, "prefixes", "sushi")
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: bin, Type: "wine"}

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := EnsurePrefix(context.Background(), r, eng, s, WinePlain, map[string]string{"WINEPREFIX": prefix}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "system.reg")); err != nil {
		t.Fatalf("registry not created: %v", err)
	}
	argv, err := os.ReadFile(filepath.Join(dir, "argv.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(argv)); got != "wineboot --init" {
		t.Fatalf("argv = %q, want %q", got, "wineboot --init")
	}
}

func TestEnsurePrefixSkipsInitialisedPrefix(t *testing.T) {
	dir := t.TempDir()
	bin := fakeWineboot(t, dir)
	prefix := filepath.Join(dir, "prefixes", "sushi")
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: bin, Type: "wine"}

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := EnsurePrefix(context.Background(), r, eng, s, WinePlain, map[string]string{"WINEPREFIX": prefix}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "argv.txt")); !os.IsNotExist(err) {
		t.Fatalf("wineboot must not run for an initialised prefix (err=%v)", err)
	}
}

func TestEnsurePrefixUsesCreateprefixForUmu(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "umu-run")
	script := "#!/bin/sh\necho \"$@\" > " + filepath.Join(dir, "argv.txt") + "\nmkdir -p \"$WINEPREFIX/pfx\"\ntouch \"$WINEPREFIX/pfx/system.reg\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	prefix := filepath.Join(dir, "prefixes", "sushi")
	s := protonSettings()
	s.WinePrefix = prefix

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := EnsurePrefix(context.Background(), r, eng, s, WineProtonUmu, map[string]string{"WINEPREFIX": prefix}); err != nil {
		t.Fatal(err)
	}
	argv, err := os.ReadFile(filepath.Join(dir, "argv.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(argv)); got != "createprefix" {
		t.Fatalf("argv = %q, want %q", got, "createprefix")
	}
	if _, err := os.Stat(filepath.Join(prefix, "pfx", "system.reg")); err != nil {
		t.Fatalf("proton registry not created: %v", err)
	}
}

func TestEnsurePrefixReportsFailure(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "wine-broken")
	script := "#!/bin/sh\necho 'wine: cannot initialise' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = filepath.Join(dir, "prefixes", "sushi")
	s.WineVersion = config.WineVersion{Bin: bin, Type: "wine"}

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	err := EnsurePrefix(context.Background(), r, eng, s, WinePlain, map[string]string{"WINEPREFIX": s.WinePrefix})
	if err == nil || !strings.Contains(err.Error(), "cannot initialise") {
		t.Fatalf("want an actionable prefix error, got %v", err)
	}
}
