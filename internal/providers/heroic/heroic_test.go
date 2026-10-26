package heroic

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hikari/internal/library"
)

const fixture = "testdata/heroic"

func game(t *testing.T, games []library.Game, store, id string) library.Game {
	t.Helper()
	for _, g := range games {
		if g.Store == store && g.ID == id {
			return g
		}
	}
	t.Fatalf("game %s/%s not found", store, id)
	return library.Game{}
}

func TestListFromLibraryCaches(t *testing.T) {
	imp := New(fixture)
	games, err := imp.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 5 {
		t.Fatalf("got %d games, want 5: %+v", len(games), games)
	}

	g := game(t, games, library.StoreEpic, "b1a2c3d4e5f6g7h8i9j0")
	if !g.Installed || g.Title != "Sushi Racer" || g.Runner != library.RunnerLegendary {
		t.Fatalf("bad epic game: %+v", g)
	}
	if g.InstallPath != "/home/tester/Games/Heroic/SushiRacer" || g.Executable != "SushiRacer/Bin/Racer.exe" {
		t.Fatalf("bad install info: %+v", g)
	}
	if g.Artwork.Cover != "https://cdn.example.test/cover.jpg" {
		t.Fatalf("bad artwork: %+v", g.Artwork)
	}
	if !g.CloudSaves || !g.CanRunOffline || g.Metadata["namespace"] != "ns-sushiracer" {
		t.Fatalf("bad flags: %+v %v", g, g.Metadata)
	}

	nori := game(t, games, library.StoreEpic, "c9d8e7f6g5h4i3j2k1l0")
	if nori.Installed {
		t.Error("Nori Puzzles should not be installed")
	}
}

func TestDLCSkipped(t *testing.T) {
	imp := New(fixture)
	games, _ := imp.List(context.Background())
	for _, g := range games {
		if g.ID == "dlc-entry-skip-me" {
			t.Fatal("DLC entry should be skipped")
		}
	}
}

func TestInstalledRecordsAugmentCaches(t *testing.T) {
	imp := New(fixture)
	games, _ := imp.List(context.Background())

	g := game(t, games, library.StoreEpic, "z9y8x7w6v5u4")
	if !g.Installed || g.InstallPath != "/home/tester/Games/Heroic/Onigiri" {
		t.Fatalf("legendary installed augmentation failed: %+v", g)
	}

	wasabi := game(t, games, library.StoreGOG, "1207658919")
	if !wasabi.Installed || wasabi.InstallPath != "/home/tester/Games/Heroic/WasabiWars" {
		t.Fatalf("gog installed augmentation failed: %+v", wasabi)
	}

	tamago := game(t, games, library.StoreAmazon, "amzn-tamago")
	if !tamago.Installed || tamago.InstallPath != "/home/tester/Games/Heroic/TamagoQuest" {
		t.Fatalf("nile installed augmentation failed: %+v", tamago)
	}
}

func TestMissingRootIsEmpty(t *testing.T) {
	imp := New(filepath.Join(t.TempDir(), "nope"))
	games, err := imp.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 0 {
		t.Fatalf("want no games, got %+v", games)
	}
}

func TestCorruptCacheFileIsSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "store_cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "store_cache", "legendary_library.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	games, err := New(root).List(context.Background())
	if err != nil {
		t.Fatalf("corrupt cache should be skipped, not fatal: %v", err)
	}
	if len(games) != 0 {
		t.Fatalf("want no games, got %+v", games)
	}
}

func TestDefaultSettingsFromGamesConfig(t *testing.T) {
	imp := New(fixture)
	g := library.Game{Store: library.StoreEpic, ID: "b1a2c3d4e5f6g7h8i9j0"}
	s, found := imp.DefaultSettings(g)
	if !found {
		t.Fatal("settings not found for fixture game")
	}
	if s.WinePrefix != "/home/tester/Games/Heroic/Prefixes/Sushi Racer" {
		t.Fatalf("bad wine prefix: %q", s.WinePrefix)
	}
	if s.WineVersion.Type != "proton" || s.WineVersion.Name != "proton-fake" {
		t.Fatalf("bad wine version: %+v", s.WineVersion)
	}
	if !s.UseGameMode || !s.ShowMangoHud || !s.EnableEsync || s.MaxSharpness != 2 {
		t.Fatalf("flags lost: %+v", s)
	}
	if len(s.Environment) != 1 || s.Environment[0].Key != "WINEDEBUG" {
		t.Fatalf("env lost: %+v", s.Environment)
	}
	if len(s.Wrappers) != 1 || s.Wrappers[0].Exe != "mangohud" {
		t.Fatalf("wrappers lost: %+v", s.Wrappers)
	}
	if s.LauncherArgs != "-windowed" {
		t.Fatalf("launcher args lost: %q", s.LauncherArgs)
	}

	if _, found := imp.DefaultSettings(library.Game{Store: library.StoreEpic, ID: "unknown"}); found {
		t.Error("unknown game should have no settings")
	}
	if _, found := imp.DefaultSettings(library.Game{Store: library.StoreSteam, ID: "b1a2c3d4e5f6g7h8i9j0"}); found {
		t.Error("steam game should have no heroic settings")
	}
}
