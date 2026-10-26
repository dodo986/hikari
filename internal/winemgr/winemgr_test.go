package winemgr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscovery(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	steam := filepath.Join(home, ".steam", "steam")

	mkFile(t, filepath.Join(data, "tools", "wine", "Wine-Test9", "bin", "wine"))
	mkFile(t, filepath.Join(data, "tools", "wine", "Wine-Test9", "bin", "wineserver"))
	os.MkdirAll(filepath.Join(data, "tools", "wine", "Wine-Test9", "lib"), 0o755)
	os.MkdirAll(filepath.Join(data, "tools", "wine", "Wine-Test9", "lib64"), 0o755)
	mkFile(t, filepath.Join(data, "tools", "proton", "GE-Proton9-99", "proton"))
	mkFile(t, filepath.Join(home, ".local", "share", "lutris", "runners", "wine", "wine-ge-8-26", "bin", "wine"))
	mkFile(t, filepath.Join(steam, "compatibilitytools.d", "GE-Proton9-20", "proton"))
	mkFile(t, filepath.Join(steam, "steamapps", "common", "Proton - Experimental", "proton"))

	os.MkdirAll(filepath.Join(steam, "compatibilitytools.d", "UMU-Latest"), 0o755)

	sysbin := filepath.Join(t.TempDir(), "wine")
	mkFile(t, sysbin)
	t.Setenv("PATH", filepath.Dir(sysbin)+":"+os.Getenv("PATH"))

	alias := filepath.Join(t.TempDir(), "steam-alias")
	if err := os.Symlink(steam, alias); err != nil {
		t.Fatal(err)
	}
	m := NewForTest(home, data, []string{steam, alias})
	versions, err := m.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Version{}
	for _, v := range versions {
		byName[v.Name] = v
	}

	wine, ok := byName["Wine-Test9"]
	if !ok || wine.Type != "wine" || wine.Source != "hikari" {
		t.Fatalf("hikari wine missing: %+v", versions)
	}
	if wine.Lib == "" || wine.Lib32 == "" || wine.Wineserver == "" {
		t.Fatalf("lib/wineserver discovery failed: %+v", wine)
	}
	if v, ok := byName["GE-Proton9-99"]; !ok || v.Type != "proton" || v.Source != "hikari" {
		t.Fatalf("hikari proton missing: %+v", versions)
	}
	if v, ok := byName["wine-ge-8-26"]; !ok || v.Source != "lutris" {
		t.Fatalf("lutris wine missing: %+v", versions)
	}
	if v, ok := byName["GE-Proton9-20"]; !ok || v.Source != "steam" {
		t.Fatalf("steam compatibility tool missing: %+v", versions)
	}
	if v, ok := byName["Proton - Experimental"]; !ok || v.Source != "steam" {
		t.Fatalf("steamapps common proton missing: %+v", versions)
	}
	if _, ok := byName["UMU-Latest"]; ok {
		t.Fatal("UMU dir must be skipped")
	}
	if v, ok := byName["system wine"]; !ok || v.Source != "system" {
		t.Fatalf("system wine missing: %+v", versions)
	}
	nGE := 0
	for _, v := range versions {
		if v.Name == "GE-Proton9-20" {
			nGE++
		}
	}
	if nGE != 1 {
		t.Fatalf("symlinked root duplicated a build (%d entries)", nGE)
	}
	if len(versions) > 1 && versions[0].Type != "proton" {
		t.Fatalf("proton should sort first: %+v", versions[0])
	}
}

func tarGz(t *testing.T, top string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	write := func(name, body string) {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write(top+"/", "")
	for name, body := range files {
		write(top+"/"+name, body)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestReleasesSelection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "proton-ge-custom"):
			json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "GE-Proton9-20", "published_at": "2025-01-02T00:00:00Z",
				"html_url": "https://example/ge",
				"assets": []map[string]any{
					{"name": "GE-Proton9-20-aarch64.tar.gz", "browser_download_url": "https://x/arm", "size": 1},
					{"name": "GE-Proton9-20.tar.gz", "browser_download_url": "https://x/x86", "size": 100},
					{"name": "GE-Proton9-20.sha512sum", "browser_download_url": "https://x/sum", "size": 10},
				},
			}})
		case strings.Contains(r.URL.Path, "proton-cachyos"):
			json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "cachyos-9.0", "published_at": "2025-02-03T00:00:00Z",
				"html_url": "https://example/cachy",
				"assets": []map[string]any{
					{"name": "proton-cachyos-9.0-x86_64.tar.xz", "browser_download_url": "https://x/cachy", "size": 200},
					{"name": "proton-cachyos-9.0-x86_64.tar.xz.sha512sum", "browser_download_url": "https://x/cachysum", "size": 10},
				},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := NewForTest(t.TempDir(), t.TempDir(), nil)
	m.apiBase = srv.URL
	rels, err := m.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 2 {
		t.Fatalf("want 2 releases, got %+v", rels)
	}
	ge := rels[0]
	if ge.Tag != "GE-Proton9-20" || ge.download != "https://x/x86" || ge.checksumURL != "https://x/sum" || ge.Date != "2025-01-02" {
		t.Fatalf("GE selection wrong: %+v", ge)
	}
	if rels[1].checksumURL != "https://x/cachysum" {
		t.Fatalf("cachy checksum wrong: %+v", rels[1])
	}
}

func TestDownloadVerifiesAndExtracts(t *testing.T) {
	archive := tarGz(t, "GE-Proton9-20", map[string]string{
		"proton":        "#!/bin/sh\necho proton",
		"files/share/x": "x",
	})
	sum := sha512.Sum512(archive)
	sumHex := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "proton-ge-custom"):
			json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "GE-Proton9-20", "published_at": "2025-01-02T00:00:00Z",
				"html_url": "https://example/ge",
				"assets": []map[string]any{
					{"name": "GE-Proton9-20.tar.gz", "browser_download_url": srvURL(r) + "/asset", "size": len(archive)},
					{"name": "GE-Proton9-20.sha512sum", "browser_download_url": srvURL(r) + "/sum", "size": len(sumHex)},
				},
			}})
		case strings.Contains(r.URL.Path, "proton-cachyos"):
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/sum":
			fmt.Fprintf(w, "%s  GE-Proton9-20.tar.gz\n", sumHex)
		case r.URL.Path == "/asset":
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	data := t.TempDir()
	m := NewForTest(t.TempDir(), data, nil)
	m.apiBase = srv.URL

	var phases []string
	name, err := m.Download(context.Background(), "GE-Proton9-20", func(p DownloadProgress) {
		phases = append(phases, p.Phase)
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "GE-Proton9-20" {
		t.Fatalf("name = %q", name)
	}
	installed := filepath.Join(data, "tools", "proton", "GE-Proton9-20")
	if _, err := os.Stat(filepath.Join(installed, "proton")); err != nil {
		t.Fatalf("proton missing after extraction: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed, "files", "share", "x")); err != nil {
		t.Fatal("archive tree not stripped/extracted correctly")
	}
	if !m.Installed("GE-Proton9-20") {
		t.Fatal("Installed() should see it")
	}
	if _, err := m.Download(context.Background(), "GE-Proton9-20", nil); err == nil {
		t.Fatal("second download should fail")
	}
	versions, _ := m.List(context.Background())
	found := false
	for _, v := range versions {
		if v.Name == "GE-Proton9-20" && v.Source == "hikari" {
			found = true
		}
	}
	if !found {
		t.Fatalf("installed version not discovered: %+v", versions)
	}
	if len(phases) == 0 || phases[len(phases)-1] != "done" {
		t.Fatalf("phases = %v", phases)
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	archive := tarGz(t, "GE-Proton9-20", map[string]string{"proton": "x"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "proton-ge-custom"):
			json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "GE-Proton9-20", "html_url": "https://example/ge",
				"assets": []map[string]any{
					{"name": "GE-Proton9-20.tar.gz", "browser_download_url": srvURL(r) + "/asset", "size": len(archive)},
					{"name": "GE-Proton9-20.sha512sum", "browser_download_url": srvURL(r) + "/sum"},
				},
			}})
		case strings.Contains(r.URL.Path, "proton-cachyos"):
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.URL.Path == "/sum":
			fmt.Fprint(w, strings.Repeat("ab", 64)+"  GE-Proton9-20.tar.gz\n")
		default:
			w.Write(archive)
		}
	}))
	defer srv.Close()

	m := NewForTest(t.TempDir(), t.TempDir(), nil)
	m.apiBase = srv.URL
	if _, err := m.Download(context.Background(), "GE-Proton9-20", nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum failure, got %v", err)
	}
}

func TestRemoveOnlyOwn(t *testing.T) {
	data := t.TempDir()
	m := NewForTest(t.TempDir(), data, nil)
	mkFile(t, filepath.Join(data, "tools", "proton", "Mine", "proton"))
	if err := m.Remove(context.Background(), "Mine"); err != nil {
		t.Fatal(err)
	}
	if m.Installed("Mine") {
		t.Fatal("remove did not delete")
	}
	if err := m.Remove(context.Background(), "SteamProton"); err == nil {
		t.Fatal("removing a non-Hikari version must fail")
	}
}

func srvURL(r *http.Request) string { return "http://" + r.Host }
