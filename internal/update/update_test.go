package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func tarball(t *testing.T, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("new binary")
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestCheckAndDownload(t *testing.T) {
	tb := tarball(t, "hikarid")
	sum := sha256.Sum256(tb)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","html_url":"https://example/rel","body":"notes","assets":[
				{"name":"hikari-v0.2.0-x86_64.tar.gz","browser_download_url":"%s/hikari-v0.2.0-x86_64.tar.gz","size":%d},
				{"name":"hikari-v0.2.0-x86_64.tar.gz.sha256","browser_download_url":"%s/hikari-v0.2.0-x86_64.tar.gz.sha256"}]}`,
				srv.URL, len(tb), srv.URL)
		case "/hikari-v0.2.0-x86_64.tar.gz":
			w.Write(tb)
		case "/hikari-v0.2.0-x86_64.tar.gz.sha256":
			fmt.Fprintf(w, "%s  hikari-v0.2.0-x86_64.tar.gz\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Checker{Repo: "o/r", Base: srv.URL, Client: srv.Client()}
	info, err := c.Check(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "v0.2.0" || info.Size == 0 {
		t.Fatalf("info = %+v", info)
	}
	dir := t.TempDir()
	var last int64
	file, err := c.Download(context.Background(), info, dir, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if last != int64(len(tb)) {
		t.Fatalf("progress = %d, want %d", last, len(tb))
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, tb) {
		t.Fatal("downloaded archive differs")
	}

	upToDate, err := c.Check(context.Background(), "0.2.0")
	if err != nil || upToDate.Available {
		t.Fatalf("same version: info=%+v err=%v", upToDate, err)
	}
	old, err := c.Check(context.Background(), "0.10.0")
	if err != nil || old.Available {
		t.Fatalf("newer local: info=%+v err=%v", old, err)
	}
}

func TestChecksumMismatchRejected(t *testing.T) {
	tb := tarball(t, "hikarid")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.3.0","assets":[
				{"name":"hikari-v0.3.0-x86_64.tar.gz","browser_download_url":"%s/hikari-v0.3.0-x86_64.tar.gz","size":%d},
				{"name":"hikari-v0.3.0-x86_64.tar.gz.sha256","browser_download_url":"%s/hikari-v0.3.0-x86_64.tar.gz.sha256"}]}`,
				srv.URL, len(tb), srv.URL)
		case "/hikari-v0.3.0-x86_64.tar.gz":
			w.Write(tb)
		case "/hikari-v0.3.0-x86_64.tar.gz.sha256":
			fmt.Fprintf(w, "%064d\n", 0)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Checker{Repo: "o/r", Base: srv.URL, Client: srv.Client()}
	info, err := c.Check(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Download(context.Background(), info, t.TempDir(), nil); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}

func TestAppImageAsset(t *testing.T) {
	body := []byte("single-file build")
	sum := sha256.Sum256(body)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.4.0","assets":[
				{"name":"hikari-v0.4.0-x86_64.tar.gz","browser_download_url":"%s/hikari-v0.4.0-x86_64.tar.gz","size":1},
				{"name":"hikari-v0.4.0-x86_64.AppImage","browser_download_url":"%s/hikari-v0.4.0-x86_64.AppImage","size":%d},
				{"name":"hikari-v0.4.0-x86_64.AppImage.sha256","browser_download_url":"%s/hikari-v0.4.0-x86_64.AppImage.sha256"}]}`,
				srv.URL, srv.URL, len(body), srv.URL)
		case "/hikari-v0.4.0-x86_64.AppImage":
			w.Write(body)
		case "/hikari-v0.4.0-x86_64.AppImage.sha256":
			fmt.Fprintf(w, "%s  hikari-v0.4.0-x86_64.AppImage\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Checker{Repo: "o/r", Base: srv.URL, Format: "appimage", Client: srv.Client()}
	info, err := c.Check(context.Background(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || !strings.HasSuffix(info.Asset, ".AppImage") {
		t.Fatalf("info = %+v", info)
	}
	file, err := c.Download(context.Background(), info, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatal("downloaded AppImage differs")
	}
}
