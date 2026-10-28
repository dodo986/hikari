package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"hikari/internal/art"
	"hikari/internal/library"
)

// Steam hands over a path inside its own librarycache, which a window cannot
// read. The daemon has to copy it into the shared cache like a fetched URL.
func TestDecorateArtworkSeedsLocalFiles(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "art")
	source := filepath.Join(root, "librarycache", "620", "library_600x900.jpg")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := &daemon{log: slog.New(slog.DiscardHandler), art: art.New(cacheDir)}
	games := []library.Game{{Store: library.StoreSteam, ID: "620", Title: "Portal 2", Artwork: library.Artwork{Square: source}}}

	d.decorateArtwork(games)

	local := games[0].ArtworkLocal.Square
	if local == "" {
		t.Fatal("artwork_local.square is empty for a local file")
	}
	if filepath.Dir(local) != cacheDir {
		t.Fatalf("artwork_local.square = %q, want a file in %q", local, cacheDir)
	}
	body, err := os.ReadFile(local)
	if err != nil || string(body) != "jpeg" {
		t.Fatalf("cached copy is %q, %v", body, err)
	}

	d.decorateArtwork(games)
	if games[0].ArtworkLocal.Square != local {
		t.Fatalf("a second pass moved the file to %q", games[0].ArtworkLocal.Square)
	}
}

func TestDecorateArtworkIgnoresMissingFiles(t *testing.T) {
	root := t.TempDir()
	d := &daemon{log: slog.New(slog.DiscardHandler), art: art.New(filepath.Join(root, "art"))}
	games := []library.Game{{Store: library.StoreSteam, ID: "1", Artwork: library.Artwork{Square: filepath.Join(root, "nope.jpg")}}}

	d.decorateArtwork(games)

	if games[0].ArtworkLocal.Square != "" {
		t.Fatalf("artwork_local.square = %q, want empty", games[0].ArtworkLocal.Square)
	}
}

func TestAnyMissingSkipsLocalFiles(t *testing.T) {
	c := art.New(t.TempDir())
	if anyMissing(c, library.Artwork{Square: "/home/someone/librarycache/620/library_600x900.jpg"}) {
		t.Fatal("a local path must not count as missing network artwork")
	}
	if !anyMissing(c, library.Artwork{Square: "https://cdn.example/620.jpg"}) {
		t.Fatal("an unfetched URL must count as missing")
	}
}
