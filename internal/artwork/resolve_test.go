package artwork

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hikari/internal/library"
)

func TestPickRejectsTheWrongGame(t *testing.T) {
	cands := []Candidate{
		{ID: "1", Name: "Doom Eternal"},
		{ID: "2", Name: "Doom Soundtrack"},
		{ID: "3", Name: "DOOM"},
	}
	best := pick("Doom", cands)
	if best == nil || best.ID != "3" {
		t.Fatalf("picked %+v, want the exact game", best)
	}
}

func TestPickAcceptsAnEditionOfTheSameGame(t *testing.T) {
	cands := []Candidate{{ID: "9", Name: "The Witcher 3: Wild Hunt - Game of the Year Edition"}}
	if best := pick("The Witcher 3 Wild Hunt", cands); best == nil || best.ID != "9" {
		t.Fatalf("picked %+v, want the GOTY entry", best)
	}
}

func TestPickAcceptsARenamedVersion(t *testing.T) {
	cands := []Candidate{
		{ID: "1", Name: "Disco Elysium - Soundtrack"},
		{ID: "2", Name: "Disco Elysium - The Final Cut"},
	}
	if best := pick("Disco Elysium", cands); best == nil || best.ID != "2" {
		t.Fatalf("picked %+v, want the final cut", best)
	}
}

func TestPickRefusesASequel(t *testing.T) {
	cands := []Candidate{{ID: "5", Name: "Doom Eternal"}}
	if best := pick("Doom", cands); best != nil {
		t.Fatalf("picked the sequel %+v", best)
	}
}

func TestPickRefusesAStranger(t *testing.T) {
	cands := []Candidate{{ID: "4", Name: "Balatro"}}
	if best := pick("Cyberpunk 2077", cands); best != nil {
		t.Fatalf("picked %+v for an unrelated title", best)
	}
}

func TestResolveBorrowsSteamArtForAStoreGame(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/storesearch/") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"items":[{"id":570,"name":"Dota 2"},{"id":1091500,"name":"Cyberpunk 2077"}]}`))
	}))
	defer srv.Close()

	s := New("")
	s.steamBase = srv.URL
	got, err := s.Resolve(context.Background(), library.Game{
		Store: library.StoreGOG, ID: "1", Title: "Cyberpunk 2077",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got.Cover, "apps/1091500/library_600x900_2x.jpg") {
		t.Fatalf("Cover = %q, want the matched portrait grid", got.Cover)
	}
	if !strings.HasSuffix(got.Square, "apps/1091500/library_hero.jpg") {
		t.Fatalf("Square = %q, want the hero banner", got.Square)
	}
	if !strings.HasSuffix(got.Logo, "apps/1091500/logo.png") {
		t.Fatalf("Logo = %q, want the wordmark", got.Logo)
	}
}

func TestResolveKeepsWhatTheStoreAlreadyHas(t *testing.T) {
	s := New("")
	g := library.Game{
		Store: library.StoreSteam, ID: "570", Title: "Dota 2",
		Artwork: library.Artwork{Cover: "/local/portrait.jpg", Square: "/local/hero.jpg", Logo: "/local/logo.png"},
	}
	if got, err := s.Resolve(context.Background(), g); err != nil || got != g.Artwork {
		t.Fatalf("resolve changed complete artwork: %+v %v", got, err)
	}
}

func TestResolveAsksSteamForTheStoreArtOfItsOwnGames(t *testing.T) {
	s := New("")
	got, _ := s.Resolve(context.Background(), library.Game{Store: library.StoreSteam, ID: "570", Title: "Dota 2"})
	if !strings.Contains(got.Cover, "apps/570/library_600x900_2x.jpg") {
		t.Fatalf("Cover = %q", got.Cover)
	}
}
