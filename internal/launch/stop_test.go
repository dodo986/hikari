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

func fakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	script := "#!/bin/sh\necho \"$@\" > " + filepath.Join(dir, name+".argv") + "\necho \"WINEPREFIX=$WINEPREFIX\" >> " + filepath.Join(dir, name+".argv") + "\necho \"STEAM_COMPAT_CLIENT_INSTALL_PATH=$STEAM_COMPAT_CLIENT_INSTALL_PATH\" >> " + filepath.Join(dir, name+".argv") + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStopPrefixPlainWine(t *testing.T) {
	dir := t.TempDir()
	wine := fakeBin(t, dir, "wine")
	fakeBin(t, dir, "wineserver")
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = t.TempDir()
	s.WineVersion = config.WineVersion{Bin: wine, Type: "wine"}

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := StopPrefix(context.Background(), r, eng, s); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "wineserver.argv"))
	if err != nil {
		t.Fatal("wineserver was not invoked")
	}
	got := string(out)
	if !strings.HasPrefix(got, "-k\n") || !strings.Contains(got, "WINEPREFIX="+s.WinePrefix) {
		t.Fatalf("wineserver argv = %q, want -k with the game prefix", got)
	}
}

func TestStopPrefixProton(t *testing.T) {
	dir := t.TempDir()
	proton := fakeBin(t, dir, "proton")
	s := protonSettings()
	s.WinePrefix = t.TempDir()
	s.WineVersion.Bin = proton

	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := StopPrefix(context.Background(), r, eng, s); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "proton.argv"))
	if err != nil {
		t.Fatal("proton was not invoked")
	}
	got := string(out)
	if !strings.HasPrefix(got, "run wineboot -k\n") {
		t.Fatalf("proton argv = %q, want run wineboot -k", got)
	}
	if !strings.Contains(got, "WINEPREFIX="+s.WinePrefix) {
		t.Fatalf("proton env lost the prefix: %q", got)
	}
	if !strings.Contains(got, "STEAM_COMPAT_CLIENT_INSTALL_PATH=") {
		t.Fatalf("proton needs STEAM_COMPAT_CLIENT_INSTALL_PATH, else it aborts: %q", got)
	}
}

func TestStopPrefixMissingPrefix(t *testing.T) {
	dir := t.TempDir()
	proton := fakeBin(t, dir, "proton")
	s := protonSettings()
	s.WinePrefix = filepath.Join(dir, "never-created")
	s.WineVersion.Bin = proton
	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	err := StopPrefix(context.Background(), r, eng, s)
	if err == nil || !strings.Contains(err.Error(), "no wine prefix") {
		t.Fatalf("err = %v, want a missing-prefix error", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "proton.argv")); !os.IsNotExist(err) {
		t.Fatal("proton ran for a prefix that does not exist")
	}
}
