package runtimes

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestEnsureDownloadsAndExtracts(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/runtimes":
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "eac_runtime", "url": "http://" + r.Host + "/eac.tar.xz", "created_at": "2024-01-14T19:38:26Z"},
				{"name": "battleye_runtime", "url": "http://" + r.Host + "/be.tar.xz", "created_at": "2024-05-21T19:35:07Z"},
			})
		case "/eac.tar.xz":
			_, _ = w.Write([]byte("runtime"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	i := NewInstaller(dir, slog.New(slog.DiscardHandler))
	i.API = srv.URL + "/runtimes"
	i.Extract = func(_ context.Context, archive, dest string) error {
		inner := filepath.Join(dest, "eac_runtime-20240114")
		if err := os.MkdirAll(inner, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(inner, "EasyAntiCheat_x64.so"), []byte("so"), 0o644)
	}

	got, err := i.Ensure(context.Background(), EAC)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "eac_runtime") {
		t.Fatalf("runtime dir = %q", got)
	}
	if _, err := os.Stat(filepath.Join(got, "EasyAntiCheat_x64.so")); err != nil {
		t.Fatalf("the archive inner directory was not flattened: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(got, "current_version"))
	if err != nil || string(b) != "2024-01-14T19:38:26Z" {
		t.Fatalf("current_version = %q, %v", b, err)
	}
	if n := requests.Load(); n != 2 {
		t.Fatalf("requests = %d, want list + archive", n)
	}

	if _, err := i.Ensure(context.Background(), EAC); err != nil {
		t.Fatal(err)
	}
	if n := requests.Load(); n != 2 {
		t.Fatalf("second Ensure made %d requests, want 2", n)
	}
}

func TestEnsureUnknownRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	i.API = srv.URL
	if _, err := i.Ensure(context.Background(), BattlEye); err == nil {
		t.Fatal("a runtime missing from the list must fail")
	}
}
