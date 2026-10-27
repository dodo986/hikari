package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	return ResolveFrom(t.TempDir(), t.TempDir())
}

func TestResolveUsesXDGLayout(t *testing.T) {
	p := ResolveFrom("/cfg/root", "/data/root")
	want := []struct{ name, got, want string }{
		{"config dir", p.ConfigDir, "/cfg/root/hikari"},
		{"data dir", p.DataDir, "/data/root/hikari"},
		{"settings", p.Settings, "/cfg/root/hikari/settings.toml"},
		{"state", p.State, "/data/root/hikari/state.json"},
		{"epic engine", p.EngineEpic, "/cfg/root/hikari/engines/legendary"},
		{"gog engine", p.EngineGOG, "/cfg/root/hikari/engines/gogdl"},
		{"amazon engine", p.EngineAmazon, "/cfg/root/hikari/engines/nile"},
		{"heroic root", p.HeroicConfig, "/cfg/root/heroic"},
	}
	for _, w := range want {
		if w.got != w.want {
			t.Errorf("%s = %q, want %q", w.name, w.got, w.want)
		}
	}
}

func TestSettingsDefaultsWhenMissing(t *testing.T) {
	p := testPaths(t)
	s, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Library.ImportHeroic {
		t.Error("import_heroic should default to true")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	p := testPaths(t)
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	in := defaultSettings()
	in.Engines.LegendaryBin = "/opt/bin/legendary"
	in.Library.HeroicConfig = "/elsewhere/heroic"
	in.Defaults.WinePrefix = "/prefixes/default"
	if err := SaveSettings(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if out.Engines.LegendaryBin != "/opt/bin/legendary" ||
		out.Library.HeroicConfig != "/elsewhere/heroic" ||
		out.Defaults.WinePrefix != "/prefixes/default" {
		t.Fatalf("round trip lost data: %+v", out)
	}
	if !out.Library.ImportHeroic {
		t.Error("import_heroic default lost after save")
	}
}

func TestSettingsToleratesUnknownKeys(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	toml := "future_key = \"whatever\"\n[engines]\nnile_bin = \"/n/nile\"\n"
	if err := os.WriteFile(p.Settings, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings(p)
	if err != nil {
		t.Fatalf("unknown key rejected: %v", err)
	}
	if s.Engines.NileBin != "/n/nile" {
		t.Fatalf("known key not loaded: %+v", s)
	}
}

func TestSettingsSaveLeavesNoTempFiles(t *testing.T) {
	p := testPaths(t)
	if err := p.Ensure(); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(p, Settings{}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(p.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestStateMissingFileIsEmpty(t *testing.T) {
	st, err := LoadState(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	if e := st.Entry("epic", "baka"); e != nil {
		t.Fatalf("empty store returned entry: %+v", e)
	}
}

func TestStateRoundTrip(t *testing.T) {
	p := testPaths(t)
	st, err := LoadState(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEntry("epic", "baka", func(e *GameEntry) {
		e.Settings.WinePrefix = "/pfx/baka"
		e.Imported = true
	}); err != nil {
		t.Fatal(err)
	}

	st2, err := LoadState(p)
	if err != nil {
		t.Fatal(err)
	}
	e := st2.Entry("epic", "baka")
	if e == nil || e.Settings.WinePrefix != "/pfx/baka" || !e.Imported {
		t.Fatalf("entry did not survive reload: %+v", e)
	}
}

func TestStateCorruptFileIsError(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(filepath.Dir(p.State), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.State, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(p); err == nil {
		t.Fatal("corrupt state should be reported, not silently reset")
	}
}

func TestSetArtworkSurvivesAFlush(t *testing.T) {
	p := Paths{State: filepath.Join(t.TempDir(), "state.json")}
	st, err := LoadState(p)
	if err != nil {
		t.Fatal(err)
	}
	st.SetArtwork("epic", "hue", ArtworkOverride{Cover: "https://cdn.example.test/portrait.jpg"})
	if err := st.Flush(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadState(p)
	if err != nil {
		t.Fatal(err)
	}
	e := again.Entry("epic", "hue")
	if e == nil || e.Artwork == nil || e.Artwork.Cover != "https://cdn.example.test/portrait.jpg" {
		t.Fatalf("artwork was not written: %+v", e)
	}
}
