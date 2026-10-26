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
	"hikari/internal/library"
)

func fakeLegendary(t *testing.T, dir, infoLine string, overlayDir string) (string, string) {
	t.Helper()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> " + calls + "\n" +
		"case \"$*\" in\n" +
		"  *install*) mkdir -p \"" + overlayDir + "\"; touch \"" + overlayDir + "/overlay.x86_64\";;\n" +
		"  *info*) echo \"" + infoLine + "\";;\n" +
		"esac\n"
	bin := filepath.Join(dir, "legendary")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

func TestPrepareEOSOverlayInstallsAndEnables(t *testing.T) {
	dir := t.TempDir()
	overlayDir := filepath.Join(dir, "eos_overlay")
	bin, calls := fakeLegendary(t, dir, "Overlay enabled: No", overlayDir)
	r := testRunnerForExtras()
	eng := engine.NewManager(config.EnginesSettings{LegendaryBin: bin}, r)
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = filepath.Join(dir, "pfx")
	s.EnableEOSOverlay = true
	g := library.Game{Store: library.StoreEpic, ID: "app"}

	PrepareEOSOverlay(context.Background(), r, eng, slog.New(slog.DiscardHandler), s, g, overlayDir, nil)

	b, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"eos-overlay info", "eos-overlay install", "eos-overlay enable"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestPrepareEOSOverlayAlreadyEnabled(t *testing.T) {
	dir := t.TempDir()
	overlayDir := filepath.Join(dir, "eos_overlay")
	bin, calls := fakeLegendary(t, dir, "Overlay enabled: Yes", overlayDir)
	r := testRunnerForExtras()
	eng := engine.NewManager(config.EnginesSettings{LegendaryBin: bin}, r)
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = filepath.Join(dir, "pfx")
	s.EnableEOSOverlay = true
	g := library.Game{Store: library.StoreEpic, ID: "app"}

	PrepareEOSOverlay(context.Background(), r, eng, slog.New(slog.DiscardHandler), s, g, overlayDir, nil)

	b, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "enable") || strings.Contains(string(b), "install") {
		t.Fatalf("already-enabled prefix was touched: %q", b)
	}
}

func TestPrepareEOSOverlaySkipped(t *testing.T) {
	dir := t.TempDir()
	bin, calls := fakeLegendary(t, dir, "Overlay enabled: No", filepath.Join(dir, "eos_overlay"))
	r := testRunnerForExtras()
	eng := engine.NewManager(config.EnginesSettings{LegendaryBin: bin}, r)
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = filepath.Join(dir, "pfx")
	log := slog.New(slog.DiscardHandler)

	PrepareEOSOverlay(context.Background(), r, eng, log, s, library.Game{Store: library.StoreEpic}, filepath.Join(dir, "o"), nil)
	s.EnableEOSOverlay = true
	PrepareEOSOverlay(context.Background(), r, eng, log, s, library.Game{Store: library.StoreGOG}, filepath.Join(dir, "o"), nil)

	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("legendary ran when it should not have (err=%v)", err)
	}
}
