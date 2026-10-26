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

func TestEnsurePrefixRepairsHalfBuiltPrefix(t *testing.T) {
	dir := t.TempDir()
	wineBin := fakeWineTree(t, true, true)
	if err := os.MkdirAll(filepath.Join(filepath.Dir(filepath.Dir(wineBin)), "lib", "wine", "i386-windows"), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(dir, "prefixes", "half")
	if err := os.MkdirAll(filepath.Join(prefix, "drive_c", "windows", "system32"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(dir, "argv.txt")
	script := "#!/bin/sh\necho \"$@\" > " + marker + "\nmkdir -p \"$WINEPREFIX/drive_c/windows/syswow64\"\n"
	if err := os.WriteFile(wineBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: wineBin, Type: "wine"}
	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := EnsurePrefix(context.Background(), r, eng, s, WinePlain, map[string]string{"WINEPREFIX": prefix}); err != nil {
		t.Fatal(err)
	}
	argv, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("wineboot must run for a half-built prefix")
	}
	if got := strings.TrimSpace(string(argv)); got != "wineboot --init" {
		t.Fatalf("argv = %q, want wineboot --init", got)
	}
	if !isDir(filepath.Join(prefix, "drive_c", "windows", "syswow64")) {
		t.Fatal("prefix was not repaired")
	}
}

func TestEnsurePrefixLeavesCompleteWow64Prefix(t *testing.T) {
	dir := t.TempDir()
	wineBin := fakeWineTree(t, true, true)
	if err := os.MkdirAll(filepath.Join(filepath.Dir(filepath.Dir(wineBin)), "lib", "wine", "i386-windows"), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(dir, "prefixes", "done")
	for _, sub := range []string{"drive_c/windows/system32", "drive_c/windows/syswow64"} {
		if err := os.MkdirAll(filepath.Join(prefix, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "argv.txt")
	script := "#!/bin/sh\necho ran > " + marker + "\n"
	if err := os.WriteFile(wineBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: wineBin, Type: "wine"}
	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	r := engine.NewRunner(slog.New(slog.DiscardHandler))
	if err := EnsurePrefix(context.Background(), r, eng, s, WinePlain, map[string]string{"WINEPREFIX": prefix}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a complete prefix must not be re-booted")
	}
}
