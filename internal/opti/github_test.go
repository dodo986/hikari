package opti

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDownloadAssetShortDigestDoesNotPanic pins down the download-mismatch
// error path: the digest comes from the external release asset, and a short
// one used to panic with "slice bounds out of range" instead of returning an
// error (killing the daemon).
func TestDownloadAssetShortDigestDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/tags/"):
			w.Write([]byte(`{"assets":[{"name":"Optiscaler_0.9.4.7z","browser_download_url":"` + srvURL(r) + `/asset","size":5,"digest":"sha256:abcd"}]}`))
		case r.URL.Path == "/asset":
			w.Write([]byte("body"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewGitHubClient()
	c.base = srv.URL + "/releases"

	dest, err := c.DownloadAsset(context.Background(), "v0.9.4", t.TempDir(), nil)
	if err == nil {
		t.Fatalf("a mismatching short digest must fail, got dest %q", dest)
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum mismatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "abcd") {
		t.Fatalf("error should name the short digest in full, got %v", err)
	}
}
