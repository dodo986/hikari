package launch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hikari/internal/config"
	"hikari/internal/dxvk"
	"hikari/internal/engine"
	"hikari/internal/runtimes"
)

type fakeTools struct {
	ensured  []dxvk.Tool
	installs []dxvk.Tool
	cached   map[dxvk.Tool]string
	due      map[dxvk.Tool]bool
	regs     []string
}

func (f *fakeTools) CheckDue(tool dxvk.Tool, maxAge time.Duration) bool { return f.due[tool] }

func (f *fakeTools) Ensure(_ context.Context, tool dxvk.Tool) (string, string, error) {
	f.ensured = append(f.ensured, tool)
	return "v1", "/cache/" + string(tool), nil
}

func (f *fakeTools) Cached(tool dxvk.Tool, version string) (string, bool) {
	if f.cached[tool] == version {
		return "/cache/" + string(tool), true
	}
	return "", false
}

func (f *fakeTools) InstallInto(ctx context.Context, prefix string, tool dxvk.Tool, version, tree string, reg dxvk.RegFunc) ([]string, error) {
	f.installs = append(f.installs, tool)
	if reg != nil {
		if err := reg(ctx, "d3d11"); err != nil {
			return nil, err
		}
	}
	return []string{"d3d11.dll"}, nil
}

func TestPrepareExtrasSkipsProton(t *testing.T) {
	tools := &fakeTools{}
	s := protonSettings()
	s.AutoInstallDxvk = true
	PrepareExtras(context.Background(), testRunnerForExtras(), nil, tools, slog.New(slog.DiscardHandler), s, WineProtonUmu, nil)
	if len(tools.ensured) != 0 || len(tools.installs) != 0 {
		t.Fatalf("proton must not get DXVK installs: %+v", tools)
	}
}

func TestPrepareExtrasInstallsForPlainWine(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "wine.args")
	wine := filepath.Join(dir, "wine")
	script := "#!/bin/sh\necho \"$@\" >> " + argsFile + "\n"
	if err := os.WriteFile(wine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := t.TempDir()
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: wine, Type: "wine"}
	s.AutoInstallDxvk = true
	s.AutoInstallVkd3d = true

	tools := &fakeTools{}
	PrepareExtras(context.Background(), testRunnerForExtras(), nil, tools, slog.New(slog.DiscardHandler), s, WinePlain, map[string]string{"WINEPREFIX": prefix})

	if len(tools.installs) != 2 {
		t.Fatalf("installs = %v, want dxvk and vkd3d", tools.installs)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("wine was never asked to register the DLL: %v", err)
	}
	if !strings.Contains(string(b), "reg add") || !strings.Contains(string(b), "d3d11") {
		t.Fatalf("wine args = %q, want reg add for d3d11", b)
	}
}

func TestPrepareExtrasUsesCachedVersion(t *testing.T) {
	prefix := t.TempDir()
	if err := os.WriteFile(filepath.Join(prefix, "current_dxvk"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: "/bin/true", Type: "wine"}
	s.AutoInstallDxvk = true
	tools := &fakeTools{cached: map[dxvk.Tool]string{dxvk.DXVK: "v1"}}
	PrepareExtras(context.Background(), testRunnerForExtras(), nil, tools, slog.New(slog.DiscardHandler), s, WinePlain, nil)
	if len(tools.ensured) != 0 {
		t.Fatalf("Ensure ran despite a cached version: %v", tools.ensured)
	}
	if len(tools.installs) != 0 {
		t.Fatalf("the same version was reinstalled: %v", tools.installs)
	}
}

func TestPrepareExtrasInstallsNewerVersion(t *testing.T) {
	prefix := t.TempDir()
	if err := os.WriteFile(filepath.Join(prefix, "current_dxvk"), []byte("v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = prefix
	s.WineVersion = config.WineVersion{Bin: "/bin/true", Type: "wine"}
	s.AutoInstallDxvk = true
	tools := &fakeTools{
		cached: map[dxvk.Tool]string{dxvk.DXVK: "v0"},
		due:    map[dxvk.Tool]bool{dxvk.DXVK: true},
	}
	PrepareExtras(context.Background(), testRunnerForExtras(), nil, tools, slog.New(slog.DiscardHandler), s, WinePlain, nil)
	if len(tools.installs) != 1 {
		t.Fatalf("a newer version must be installed: %v", tools.installs)
	}
}

func testRunnerForExtras() *engine.Runner {
	return engine.NewRunner(slog.New(slog.DiscardHandler))
}

type fakeRuntimes struct {
	installed map[runtimes.Name]bool
	ensured   []runtimes.Name
}

func (f *fakeRuntimes) Installed(name runtimes.Name) (string, bool) {
	return "/rt/" + string(name), f.installed[name]
}

func (f *fakeRuntimes) Ensure(_ context.Context, name runtimes.Name) (string, error) {
	f.ensured = append(f.ensured, name)
	return "/rt/" + string(name), nil
}

func TestPrepareRuntimesOnlyWhenMissing(t *testing.T) {
	s := FactorySettings(config.DefaultsSettings{})
	s.EacRuntime = true
	s.BattlEyeRuntime = true
	log := slog.New(slog.DiscardHandler)

	f := &fakeRuntimes{installed: map[runtimes.Name]bool{runtimes.EAC: true}}
	PrepareRuntimes(context.Background(), f, log, s, WineProtonUmu)
	if len(f.ensured) != 1 || f.ensured[0] != runtimes.BattlEye {
		t.Fatalf("ensured = %v, want just battleye", f.ensured)
	}

	f = &fakeRuntimes{}
	PrepareRuntimes(context.Background(), f, log, s, WineNative)
	if len(f.ensured) != 0 {
		t.Fatalf("native launch fetched runtimes: %v", f.ensured)
	}
}
