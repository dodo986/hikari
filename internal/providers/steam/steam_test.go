package steam

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/library"
)

func fixtureRoots(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	r1 := filepath.Join(dir, "root1")
	r2 := filepath.Join(dir, "root2")
	copyTree(t, "testdata/root1", r1)
	copyTree(t, "testdata/root2", r2)
	rewrite := func(path string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ReplaceAll(string(b), "REPLACED_ROOT1", r1)
		s = strings.ReplaceAll(s, "REPLACED_ROOT2", r2)
		if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rewrite(filepath.Join(r1, "steamapps", "libraryfolders.vdf"))
	return []string{r1}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListAcrossLibraries(t *testing.T) {
	p := NewWithRoots(fixtureRoots(t)...)
	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 {
		t.Fatalf("want 2 games, got %+v", games)
	}
	var hl, sos library.Game
	for _, g := range games {
		switch g.ID {
		case "546050":
			hl = g
		case "1593500":
			sos = g
		}
	}
	if hl.Title != "Half-Life 2: Ep One" || !hl.Installed {
		t.Fatalf("installed game wrong: %+v", hl)
	}
	if hl.Store != library.StoreSteam || hl.Runner != library.RunnerSteam {
		t.Fatalf("identity wrong: %+v", hl)
	}
	if !strings.Contains(hl.InstallPath, "steamapps/common/Half-Life 2 Episode One") {
		t.Fatalf("install path: %q", hl.InstallPath)
	}
	if !strings.HasSuffix(hl.Artwork.Cover, "librarycache/546050/library_600x900.jpg") {
		t.Fatalf("the portrait cache belongs in Cover: %+v", hl.Artwork)
	}
	if !strings.HasSuffix(hl.Artwork.Logo, "apps/546050/logo.png") {
		t.Fatalf("the wordmark comes from the CDN: %+v", hl.Artwork)
	}
	if sos.Installed {
		t.Fatalf("downloading game must not be installed: %+v", sos)
	}
	if sos.Title != "Sea of Stars" {
		t.Fatalf("title: %q", sos.Title)
	}
}

func TestMissingRootIsQuiet(t *testing.T) {
	p := NewWithRoots(filepath.Join(t.TempDir(), "absent"))
	games, err := p.List(context.Background())
	if err != nil || len(games) != 0 {
		t.Fatalf("absent root: %+v %v", games, err)
	}
}

func TestParseVDF(t *testing.T) {
	doc, err := parseVDF(`"root" { "k" "v" "n" { "deep" "1" } }`)
	if err != nil {
		t.Fatal(err)
	}
	if vdfString(doc, "root", "k") != "v" {
		t.Fatal("flat value")
	}
	if vdfString(doc, "root", "n", "deep") != "1" {
		t.Fatal("nested value")
	}
	if _, err := parseVDF(`"a" "b" }`); err == nil {
		t.Fatal("trailing junk must error")
	}
	if _, err := parseVDF(`{ "a" "unterminated }`); err == nil {
		t.Fatal("unterminated string must error")
	}
	doc, err = parseVDF("{ // line comment\n \"a\" bare\n}")
	if err != nil {
		t.Fatal(err)
	}
	if vdfString(doc, "a") != "bare" {
		t.Fatalf("bare value: %q", vdfString(doc, "a"))
	}
}

func TestLaunchOptionsHint(t *testing.T) {
	if got := LaunchOptions("dxgi.dll"); got != "WINEDLLOVERRIDES=dxgi=n,b %command%" {
		t.Fatalf("launch options: %q", got)
	}
	if got := LaunchOptions("winmm.dll"); got != "WINEDLLOVERRIDES=winmm=n,b %command%" {
		t.Fatalf("launch options: %q", got)
	}
	if got := LaunchOptions(""); got != "WINEDLLOVERRIDES=dxgi=n,b %command%" {
		t.Fatalf("default: %q", got)
	}
}

func TestSteamArtFindsEveryCacheLayout(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "appcache", "librarycache", "5172140")
	hash := filepath.Join(app, "33ce64b175c8ac728007a4024bfca399201d5e98")
	if err := os.MkdirAll(hash, 0o755); err != nil {
		t.Fatal(err)
	}
	portrait := filepath.Join(app, "library_600x900.jpg")
	header := filepath.Join(hash, "library_header.jpg")
	for _, p := range []string{portrait, header} {
		if err := os.WriteFile(p, []byte("jpg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	art := steamArt(root, "5172140")
	if art.Cover != portrait {
		t.Errorf("Cover = %q, want the flat portrait", art.Cover)
	}
	if art.Square != header {
		t.Errorf("Square = %q, want the hashed header", art.Square)
	}

	os.Remove(portrait)
	os.Remove(header)
	art = steamArt(root, "5172140")
	if !strings.HasSuffix(art.Cover, "apps/5172140/library_600x900_2x.jpg") {
		t.Errorf("Cover = %q, want the double resolution CDN grid", art.Cover)
	}

	art = steamArt(t.TempDir(), "999")
	if !strings.HasPrefix(art.Cover, "https://cdn.akamai.steamstatic.com/steam/apps/999/") ||
		!strings.HasPrefix(art.Square, "https://cdn.akamai.steamstatic.com/steam/apps/999/") {
		t.Errorf("no-cache art = %+v, want CDN URLs", art)
	}
}
