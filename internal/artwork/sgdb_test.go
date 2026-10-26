package artwork

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSGDBFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/search/autocomplete/"):
			w.Write([]byte(`{"success":true,"data":[{"id":7,"name":"Fall Guys"}]}`))
		case strings.Contains(r.URL.Path, "/grids/game/7"):
			if r.URL.Query().Get("dimensions") != "600x900" {
				t.Errorf("dimensions filter missing: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"success":true,"data":[{"url":"https://cdn.sgdb/1.png","thumb":"https://cdn.sgdb/t1.png","width":600,"height":900}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s := New("key123")
	s.sgdbBase = srv.URL
	cands, err := s.Search(context.Background(), "Fall Guys")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Name != "Fall Guys" || cands[0].Source != "steamgriddb" {
		t.Fatalf("candidates: %+v", cands)
	}
	if len(cands[0].Covers) != 1 || cands[0].Covers[0].URL != "https://cdn.sgdb/1.png" {
		t.Fatalf("covers: %+v", cands[0].Covers)
	}
}

func TestKeylessSteamFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/storesearch/") || r.URL.Query().Get("term") != "Fall Guys" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"total":1,"items":[{"id":1097150,"name":"Fall Guys","tiny_image":"https://cdn.steam/tiny.jpg"}]}`))
	}))
	defer srv.Close()

	s := New("")
	s.steamBase = srv.URL
	cands, err := s.Search(context.Background(), "Fall Guys")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].ID != "1097150" || cands[0].Source != "steam" {
		t.Fatalf("candidates: %+v", cands)
	}
	if len(cands[0].Covers) < 1 || !strings.Contains(cands[0].Covers[0].URL, "1097150/header.jpg") {
		t.Fatalf("covers: %+v", cands[0].Covers)
	}
}

func TestBadKeyFallsBack(t *testing.T) {

	sgdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer sgdb.Close()
	steam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"total":1,"items":[{"id":1,"name":"X","tiny_image":"t"}]}`))
	}))
	defer steam.Close()

	s := New("bad")
	s.sgdbBase = sgdb.URL
	s.steamBase = steam.URL
	cands, err := s.Search(context.Background(), "X")
	if err != nil || len(cands) != 1 || cands[0].Source != "steam" {
		t.Fatalf("fallback failed: %+v %v", cands, err)
	}
}

func TestEmptyTitle(t *testing.T) {
	if _, err := New("").Search(context.Background(), "   "); err == nil {
		t.Fatal("empty title must error")
	}
}
