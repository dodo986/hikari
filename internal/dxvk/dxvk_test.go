package dxvk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func fakeExtract(t *testing.T, version, x86Dir string) func(context.Context, string, string) error {
	return func(_ context.Context, archive, dest string) error {
		if _, err := os.Stat(archive); err != nil {
			t.Errorf("archive was not downloaded: %v", err)
		}
		for _, arch := range []string{"x64", x86Dir} {
			dir := filepath.Join(dest, "tool-"+version, arch)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, "d3d11.dll"), []byte(arch), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestEnsureDownloadsAndCaches(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/repos/doitsujin/dxvk/releases/latest":
			body := map[string]any{
				"tag_name": "v9.9.9",
				"assets": []map[string]any{
					{"name": "dxvk-native-9.9.9-steamrt.tar.gz", "browser_download_url": "http://" + r.Host + "/native", "size": 3},
					{"name": "dxvk-9.9.9.tar.gz", "browser_download_url": "http://" + r.Host + "/dxvk", "size": 3},
				},
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/dxvk":
			_, _ = w.Write([]byte("abc"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	i.APIBase = srv.URL
	i.Extract = fakeExtract(t, "v9.9.9", "x32")

	version, tree, err := i.Ensure(context.Background(), DXVK)
	if err != nil {
		t.Fatal(err)
	}
	if version != "v9.9.9" {
		t.Fatalf("version = %q", version)
	}
	if _, _, ok := Locate(DXVK, tree); !ok {
		t.Fatalf("no tree at %s", tree)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want API + asset", got)
	}

	if _, _, err := i.Ensure(context.Background(), DXVK); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("cached Ensure made %d requests, want 3 (API only, no download)", got)
	}
	if _, ok := i.Cached(DXVK, version); !ok {
		t.Fatal("Cached should find the extracted tree")
	}
	if i.CheckDue(DXVK, 24*time.Hour) {
		t.Fatal("a successful Ensure must stamp the check")
	}
}

func TestEnsureUnknownTool(t *testing.T) {
	i := NewInstaller(t.TempDir(), nil)
	if _, _, err := i.Ensure(context.Background(), Tool("mesa")); err == nil {
		t.Fatal("unknown tool must fail")
	}
}

func TestEnsureRejectsMissingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1","assets":[{"name":"source.zip","browser_download_url":"http://x/y","size":1}]}`)
	}))
	defer srv.Close()
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	i.APIBase = srv.URL
	if _, _, err := i.Ensure(context.Background(), DXVK); err == nil {
		t.Fatal("a release without the expected asset must fail")
	}
}

// TestEnsureDoesNotCachePartialExtraction guards against a half-extracted tree
// being published to the version cache: extraction lands in a temp sibling and
// is only renamed into place once a full tree is there.
func TestEnsureDoesNotCachePartialExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/doitsujin/dxvk/releases/latest":
			body := map[string]any{
				"tag_name": "v9.9.9",
				"assets": []map[string]any{
					{"name": "dxvk-9.9.9.tar.gz", "browser_download_url": "http://" + r.Host + "/dxvk", "size": 3},
				},
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/dxvk":
			_, _ = w.Write([]byte("abc"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	i.APIBase = srv.URL
	var extracts atomic.Int32
	i.Extract = func(_ context.Context, archive, dest string) error {
		extracts.Add(1)
		if _, err := os.Stat(archive); err != nil {
			t.Errorf("archive was not downloaded: %v", err)
		}
		x64 := filepath.Join(dest, "tool-v9.9.9", "x64")
		if err := os.MkdirAll(x64, 0o755); err != nil {
			return err
		}
		payload := "partial"
		if extracts.Load() > 1 {
			payload = "full"
		}
		if err := os.WriteFile(filepath.Join(x64, "d3d11.dll"), []byte(payload), 0o644); err != nil {
			return err
		}
		if extracts.Load() == 1 {
			// Simulate a crash or a full disk halfway through extraction.
			return errors.New("simulated extraction failure")
		}
		x32 := filepath.Join(dest, "tool-v9.9.9", "x32")
		if err := os.MkdirAll(x32, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(x32, "d3d11.dll"), []byte(payload), 0o644)
	}

	if _, _, err := i.Ensure(context.Background(), DXVK); err == nil {
		t.Fatal("Ensure must report the extraction failure")
	}
	dir := i.cachePath(DXVK, "v9.9.9")
	if _, _, ok := Locate(DXVK, dir); ok {
		t.Fatalf("a partial tree was left at the cache path %s", dir)
	}
	if _, ok := i.Cached(DXVK, "v9.9.9"); ok {
		t.Fatal("Cached must not accept a failed extraction")
	}
	if _, err := os.Stat(dir + ".extract"); !os.IsNotExist(err) {
		t.Fatalf("extraction temp dir was not cleaned up: %v", err)
	}

	version, tree, err := i.Ensure(context.Background(), DXVK)
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if version != "v9.9.9" {
		t.Fatalf("version = %q, want v9.9.9", version)
	}
	if tree != dir {
		t.Fatalf("tree = %q, want the cache path %q", tree, dir)
	}
	if got := extracts.Load(); got != 2 {
		t.Fatalf("Extract called %d times, want 2: the failed attempt must not be reused", got)
	}
	if _, _, ok := Locate(DXVK, tree); !ok {
		t.Fatalf("no usable tree at %s", tree)
	}
	b, err := os.ReadFile(filepath.Join(tree, "tool-v9.9.9", "x64", "d3d11.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "full" {
		t.Fatalf("cached DLL = %q, want the freshly extracted %q", b, "full")
	}
}
