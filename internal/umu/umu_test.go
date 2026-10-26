package umu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFromAPIAndCache(t *testing.T) {
	var apiCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		if r.URL.Query().Get("codename") != "baka" || r.URL.Query().Get("store") != "egs" {
			t.Errorf("bad query: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`[{"title":"Baka","umu_id":"umu-1234"}]`))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "umu_ids.json")
	r := NewResolver(path)
	r.base = srv.URL

	id, err := r.Resolve(context.Background(), "epic", "baka")
	if err != nil || id != "umu-1234" {
		t.Fatalf("resolve = %q, %v", id, err)
	}
	r2 := NewResolver(path)
	r2.base = "http://127.0.0.1:1"
	id, err = r2.Resolve(context.Background(), "epic", "baka")
	if err != nil || id != "umu-1234" {
		t.Fatalf("cached resolve = %q, %v", id, err)
	}
	if apiCalls != 1 {
		t.Fatalf("api called %d times, want 1", apiCalls)
	}
}

func TestMissIsCached(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "umu_ids.json")
	r := NewResolver(path)
	r.base = srv.URL
	for i := 0; i < 2; i++ {
		id, err := r.Resolve(context.Background(), "gog", "unknown-game")
		if err != nil || id != "" {
			t.Fatalf("miss resolve = %q, %v", id, err)
		}
	}
	if calls != 1 {
		t.Fatalf("miss should be cached, api called %d times", calls)
	}
}

func TestNetworkFailureIsNotFatal(t *testing.T) {
	r := NewResolver(filepath.Join(t.TempDir(), "umu_ids.json"))
	r.base = "http://127.0.0.1:1"
	id, err := r.Resolve(context.Background(), "epic", "baka")
	if id != "" {
		t.Fatalf("want empty id on failure, got %q", id)
	}
	if err == nil {
		t.Fatal("failure should be reported as an error (caller logs it)")
	}

	if _, err := os.Stat(r.path); !os.IsNotExist(err) {
		t.Fatal("a network failure must not persist any cache")
	}
}

func TestUnsupportedStoreAndEmptyApp(t *testing.T) {
	r := NewResolver(filepath.Join(t.TempDir(), "umu_ids.json"))
	if id, err := r.Resolve(context.Background(), "steam", "x"); id != "" || err != nil {
		t.Fatalf("steam should be a no-op: %q %v", id, err)
	}
	if id, err := r.Resolve(context.Background(), "epic", ""); id != "" || err != nil {
		t.Fatalf("empty app should be a no-op: %q %v", id, err)
	}
}
