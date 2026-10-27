package gog

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

const galaxyPage1 = `{
  "items": [
    { "platform_id": "gog", "external_id": "1207658999", "owned": true, "certificate": "cert-1" },
    { "platform_id": "movie", "external_id": "9", "owned": true }
  ],
  "next_page_token": "page2"
}`

const galaxyPage2 = `{
  "items": [
    { "platform_id": "gog", "external_id": "1423049311", "owned": true, "certificate": "cert-2" }
  ]
}`

const gamesdbGame = `{
  "type": "game",
  "external_id": "1207658999",
  "title": { "*": "Wasabi Wars" },
  "game": {
    "visible_in_library": true,
    "title": { "*": "Wasabi Wars" },
    "developers": [ { "name": "Fake Studio" } ],
    "logo": { "url_format": "https://images.gog.com/{formatter}/logo.{ext}" },
    "vertical_cover": { "url_format": "https://images.gog.com/{formatter}/cover.{ext}" },
    "square_icon": { "url_format": "https://images.gog.com/{formatter}/icon.{ext}" },
    "background": { "url_format": "https://images.gog.com/{formatter}/bg.{ext}" },
    "genres": [ { "name": { "*": "Racing" } } ]
  },
  "supported_operating_systems": [ { "slug": "windows" }, { "slug": "linux" } ]
}`

const gamesdbCyberpunk = `{
  "type": "game",
  "external_id": "1423049311",
  "title": { "*": "Mochi Mansion 2077" },
  "game": {
    "visible_in_library": true,
    "title": { "*": "Mochi Mansion 2077" },
    "developers": [ { "name": "Another Studio" } ]
  },
  "supported_operating_systems": [ { "slug": "windows" } ]
}`

const gamesdbDLC = `{
  "type": "dlc",
  "external_id": "whatever",
  "game": { "visible_in_library": true, "title": { "*": "DLC" } }
}`

func newGalaxyTestProvider(t *testing.T) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/users/u-7/releases"):
			if r.URL.Query().Get("page_token") == "" {
				w.Write([]byte(galaxyPage1))
			} else {
				w.Write([]byte(galaxyPage2))
			}
		case strings.Contains(r.URL.Path, "/external_releases/1207658999"):
			if r.Header.Get("X-GOG-Library-Cert") != "cert-1" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(gamesdbGame))
		case strings.Contains(r.URL.Path, "/external_releases/1423049311"):
			w.Write([]byte(gamesdbCyberpunk))
		case strings.Contains(r.URL.Path, "/external_releases/"):
			w.Write([]byte(gamesdbDLC))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	bin := filepath.Join(dir, "gogdl")
	os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	paths.Ensure()
	eng := engine.NewManager(config.EnginesSettings{GogdlBin: bin}, nil)
	p := NewWithGalaxy(eng, engine.NewRunner(discardLogger()), paths, NewGalaxyClient())
	p.galaxy.libBase = srv.URL
	p.galaxy.dbBase = srv.URL
	os.WriteFile(filepath.Join(paths.EngineGOG, "auth.json"),
		[]byte(`{"access_token":"tok","refresh_token":"r","user_id":"u-7"}`), 0o600)
	return p, srv
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func discardRunner() *engine.Runner { return engine.NewRunner(discardLogger()) }

func TestGalaxyRefreshPaginatesAndMerges(t *testing.T) {
	p, _ := newGalaxyTestProvider(t)

	p.upsertCache(entry{
		ID: "1207658999", Title: "Wasabi Wars (old)", Installed: true,
		InstallPath: "/games/WasabiWars", Platform: "windows", Version: "2.0",
	})

	n, err := p.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("refreshed %d games, want 2", n)
	}

	games, _ := p.List(context.Background())
	if len(games) != 2 {
		t.Fatalf("library: %+v", games)
	}
	var ww library.Game
	for _, g := range games {
		if g.ID == "1207658999" {
			ww = g
		}
	}
	if !ww.Installed || ww.InstallPath != "/games/WasabiWars" || ww.Version != "2.0" {
		t.Fatalf("install state lost: %+v", ww)
	}
	if ww.Developer != "Fake Studio" || ww.Title != "Wasabi Wars" {
		t.Fatalf("enrichment lost: %+v", ww)
	}
	if ww.Artwork.Cover != "https://images.gog.com//cover.jpg" {
		t.Fatalf("portrait art must go to Cover: %+v", ww.Artwork)
	}
	if ww.Artwork.Square != "https://images.gog.com//bg.webp" {
		t.Fatalf("the wide background must go to Square: %+v", ww.Artwork)
	}
	var mm library.Game
	for _, g := range games {
		if g.ID == "1423049311" {
			mm = g
		}
	}
	if mm.Installed || mm.Title != "Mochi Mansion 2077" || mm.Platform != "Windows" {
		t.Fatalf("page 2 game: %+v", mm)
	}

	for _, g := range games {
		if g.ID == "9" || g.ID == "whatever" {
			t.Fatalf("filtered entry leaked: %+v", g)
		}
	}
	games2, _ := p.List(context.Background())
	if len(games2) != 2 {
		t.Fatalf("cache did not persist: %+v", games2)
	}
}

func TestUsernameResolvesAndCaches(t *testing.T) {
	p, _ := newGalaxyTestProvider(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"username":"FakeGogUser","userId":"u-7"}`))
	}))
	defer srv.Close()
	old := usersBase
	usersBase = srv.URL
	defer func() { usersBase = old }()

	name, err := p.Username(context.Background())
	if err != nil || name != "FakeGogUser" {
		t.Fatalf("username = %q, %v", name, err)
	}
	name, err = p.Username(context.Background())
	if err != nil || name != "FakeGogUser" || calls != 1 {
		t.Fatalf("cache miss: %q %v calls=%d", name, err, calls)
	}
}

func TestGalaxyRefreshNotLoggedIn(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gogdl")
	os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	paths.Ensure()
	eng := engine.NewManager(config.EnginesSettings{GogdlBin: bin}, nil)
	p := New(eng, discardRunner(), paths)
	p.galaxy = NewGalaxyClient()
	p.galaxy.libBase = "http://127.0.0.1:1"
	p.galaxy.dbBase = "http://127.0.0.1:1"
	if _, err := p.Refresh(context.Background()); err != nil {
		t.Fatalf("logged-out refresh must fall back to the cache: %v", err)
	}
}

func TestGalaxyEmptyLibraryKeepsCache(t *testing.T) {
	p, _ := newGalaxyTestProvider(t)
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/users/") {
			w.Write([]byte(`{"items":[]}`))
			return
		}
		w.Write([]byte(gamesdbDLC))
	}))
	defer empty.Close()
	p.galaxy.libBase = empty.URL
	p.galaxy.dbBase = empty.URL

	p.upsertCache(entry{ID: "1", Title: "Kept", Installed: true, InstallPath: "/games/kept"})
	n, err := p.Refresh(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("installed record lost on empty response: n=%d err=%v", n, err)
	}
	games, _ := p.List(context.Background())
	if len(games) != 1 || games[0].Title != "Kept" || !games[0].Installed {
		t.Fatalf("cache was wiped: %+v", games)
	}

	p2, _ := newGalaxyTestProvider(t)
	p2.galaxy.libBase = empty.URL
	p2.galaxy.dbBase = empty.URL
	if _, err := p2.Refresh(context.Background()); err == nil {
		t.Fatal("an empty library with nothing installed should report an error")
	}
}
