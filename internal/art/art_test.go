package art

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testGetter struct {
	server *httptest.Server
	calls  int
}

func (g *testGetter) Do(req *http.Request) (*http.Response, error) {
	g.calls++
	return g.server.Client().Do(req)
}

// Get keeps the fake usable against the pre-fix Getter interface, so the
// context regression test below fails on old code instead of not compiling.
func (g *testGetter) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return g.Do(req)
}

// blockingGetter ignores a cancelled context on the pre-fix Get path and
// returns as soon as the request context is done on the new Do path.
type blockingGetter struct {
	entered chan struct{}
	release chan struct{}
	seen    chan error
}

func (g *blockingGetter) enter() {
	select {
	case g.entered <- struct{}{}:
	default:
	}
}

func (g *blockingGetter) Do(req *http.Request) (*http.Response, error) {
	g.enter()
	select {
	case <-req.Context().Done():
	case <-g.release:
	}
	err := req.Context().Err()
	if err != nil {
		select {
		case g.seen <- err:
		default:
		}
	}
	return nil, err
}

func (g *blockingGetter) Get(string) (*http.Response, error) {
	g.enter()
	<-g.release
	return nil, context.Canceled
}

func newTestCache(t *testing.T) (*Cache, *testGetter) {
	t.Helper()
	g := &testGetter{}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cover.jpg":
			w.Write([]byte(strings.Repeat("x", 1024)))
		case "/huge.jpg":
			w.Write([]byte(strings.Repeat("x", DefaultMaxImage+10)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(g.server.Close)
	return NewWithGetter(t.TempDir(), g), g
}

func TestFetchCachesAndSecondIsFree(t *testing.T) {
	c, g := newTestCache(t)
	p1, err := c.Fetch(context.Background(), g.server.URL+"/cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(p1) != ".jpg" {
		t.Fatalf("ext = %q", filepath.Ext(p1))
	}
	if _, err := os.Stat(p1); err != nil {
		t.Fatal(err)
	}
	p2, err := c.Fetch(context.Background(), g.server.URL+"/cover.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("path changed: %q vs %q", p1, p2)
	}
	if g.calls != 1 {
		t.Fatalf("second fetch hit the network (%d calls)", g.calls)
	}
}

func TestOversizedImageRejected(t *testing.T) {
	c, _ := newTestCache(t)
	if _, err := c.Fetch(context.Background(), "whatever:///nope"); err == nil {
		t.Fatal("non-http URL must be rejected")
	}
	c2, g := newTestCache(t)
	if _, err := c2.Fetch(context.Background(), g.server.URL+"/huge.jpg"); err == nil {
		t.Fatal("oversized image must be rejected")
	}
	entries, _ := os.ReadDir(c2.dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".fetch-") || strings.HasPrefix(e.Name(), "huge") {
			t.Fatalf("temp/leftover file: %s", e.Name())
		}
	}
	if c2.Has(g.server.URL + "/huge.jpg") {
		t.Fatal("rejected image must not count as cached")
	}
}

func TestBadStatusAndMissing(t *testing.T) {
	c, g := newTestCache(t)
	if _, err := c.Fetch(context.Background(), g.server.URL+"/missing.jpg"); err == nil {
		t.Fatal("404 must be an error")
	}
}

func TestEvictionKeepsCacheBounded(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	c.maxImageBytes = 1 << 20
	c.maxTotalBytes = 3 << 10

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%-1024s", r.URL.Path)
	}))
	defer srv.Close()
	c.client = &testGetter{server: srv}

	var paths []string
	for i := 0; i < 5; i++ {
		p, err := c.Fetch(context.Background(), fmt.Sprintf("%s/%d.jpg", srv.URL, i))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
		future := timeNow().Add(time.Duration(i) * time.Hour)
		os.Chtimes(p, future, future)
	}
	if _, err := c.Fetch(context.Background(), fmt.Sprintf("%s/%d.jpg", srv.URL, 4)); err != nil {
		t.Fatal(err)
	}
	var total int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		info, _ := e.Info()
		total += info.Size()
	}
	if total > c.maxTotalBytes {
		t.Fatalf("cache over budget: %d > %d", total, c.maxTotalBytes)
	}
	if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
		t.Fatal("oldest image should have been evicted")
	}
}

func TestExtSafety(t *testing.T) {
	c := New(t.TempDir())
	if p := c.PathFor("https://x/cover.PNG?token=1"); !strings.HasSuffix(p, ".png") {
		t.Fatalf("ext not normalised: %q", p)
	}
	if p := c.PathFor("https://x/cover.exe"); !strings.HasSuffix(p, ".jpg") {
		t.Fatalf("unsafe ext must fall back to .jpg: %q", p)
	}
	if p := c.PathFor("file:///etc/passwd"); p != "" {
		t.Fatalf("file URL must be refused: %q", p)
	}
}

var timeNow = func() time.Time { return time.Now().UTC() }

func TestFetchHonoursRequestContext(t *testing.T) {
	g := &blockingGetter{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
		seen:    make(chan error, 1),
	}
	defer close(g.release)
	c := NewWithGetter(t.TempDir(), g)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.Fetch(ctx, "https://art.example/cover.jpg")
		done <- err
	}()

	select {
	case <-g.entered:
	case <-time.After(time.Second):
		t.Fatal("the getter was never called")
	}
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Fetch returned success after the context was cancelled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Fetch ignored the cancelled request context")
	}
	select {
	case got := <-g.seen:
		if !errors.Is(got, context.Canceled) {
			t.Fatalf("getter saw %v, want context.Canceled", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the getter never observed the cancelled request context")
	}
}

// errReader fails after the caller has copied whatever the wrapped readers
// already produced, which is what a half-written seed looks like.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestSeedLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	url := "https://art.example/cover.jpg"
	if err := c.Seed(url, io.MultiReader(strings.NewReader("partial"), errReader{})); err == nil {
		t.Fatal("Seed must surface the copy error")
	}
	if _, err := os.Stat(c.PathFor(url)); !os.IsNotExist(err) {
		t.Fatal("Seed left a partial cache file behind")
	}
	assertDirEmpty(t, dir)
}

func TestSeedFileLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	p, err := c.SeedFile(filepath.Join(dir, "picked.png"), io.MultiReader(strings.NewReader("partial"), errReader{}))
	if err == nil {
		t.Fatalf("SeedFile must surface the copy error, got %q", p)
	}
	if p != "" {
		t.Fatalf("SeedFile returned %q on error", p)
	}
	assertDirEmpty(t, dir)
}

func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Fatalf("leftover file after a failed write: %s", e.Name())
	}
}
