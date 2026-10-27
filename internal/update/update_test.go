package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commitsServer(t *testing.T, shas []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/dodo986/hikari/commits") {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("sha"); got != "main" {
			t.Errorf("asked for branch %q", got)
		}
		var out []map[string]any
		for _, sha := range shas {
			out = append(out, map[string]any{
				"sha": sha,
				"commit": map[string]any{
					"message": "fix: something\n\nbody that is not shown",
					"author":  map[string]any{"date": "2026-10-27T10:00:00Z"},
				},
			})
		}
		json.NewEncoder(w).Encode(out)
	}))
}

func TestCheckCountsTheCommitsSinceTheBuild(t *testing.T) {
	server := commitsServer(t, []string{"cccc2222aaaa", "bbbb1111aaaa", "aaaa0000aaaa"})
	defer server.Close()
	checker := New("dodo986/hikari")
	checker.Base = server.URL

	info, err := checker.Check(context.Background(), "0.8.2", "aaaa0000")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Behind != 2 {
		t.Fatalf("expected to be 2 commits behind, got available=%v behind=%d", info.Available, info.Behind)
	}
	if info.Latest != "cccc2222" || info.Sha != "cccc2222aaaa" {
		t.Fatalf("latest = %q sha = %q", info.Latest, info.Sha)
	}
	if !strings.Contains(info.Notes, "fix: something") || strings.Contains(info.Notes, "body that is not shown") {
		t.Fatalf("notes = %q", info.Notes)
	}
	if !strings.Contains(info.URL, "compare/aaaa0000...main") {
		t.Fatalf("url = %q", info.URL)
	}
}

func TestCheckIsQuietOnTheHeadCommit(t *testing.T) {
	server := commitsServer(t, []string{"cccc2222aaaa"})
	defer server.Close()
	checker := New("")
	checker.Base = server.URL

	info, err := checker.Check(context.Background(), "0.8.2", "cccc2222")
	if err != nil {
		t.Fatal(err)
	}
	if info.Available {
		t.Fatal("the head commit must not look like an update")
	}
}

func TestCheckReportsAnUnstampedBuildWithoutOfferingOne(t *testing.T) {
	server := commitsServer(t, []string{"cccc2222aaaa"})
	defer server.Close()
	checker := New("")
	checker.Base = server.URL

	info, err := checker.Check(context.Background(), "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Available || info.Latest != "cccc2222" {
		t.Fatalf("a build with no commit stamp should only report the head: %+v", info)
	}
}

func TestCheckAcceptsAFullCommitAgainstAShortOne(t *testing.T) {
	server := commitsServer(t, []string{"cccc2222deadbeefdeadbeefdeadbeefdeadbeef"})
	defer server.Close()
	checker := New("")
	checker.Base = server.URL

	info, err := checker.Check(context.Background(), "0.8.2", "cccc2222")
	if err != nil {
		t.Fatal(err)
	}
	if info.Available {
		t.Fatal("a short prefix of the head commit must match it")
	}
}

func TestSourceFetchesTheCommitArchive(t *testing.T) {
	body := []byte("pretend tarball")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dodo986/hikari/tar.gz/cccc2222aaaa" {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	defer server.Close()
	checker := New("")
	checker.Codeload = server.URL

	dir := t.TempDir()
	var seen int64
	path, sum, err := checker.Source(context.Background(), Info{Sha: "cccc2222aaaa"}, dir, func(done int64) { seen = done })
	if err != nil {
		t.Fatal(err)
	}
	if seen != int64(len(body)) {
		t.Fatalf("progress reported %d of %d bytes", seen, len(body))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("saved %q", got)
	}
	if len(sum) != 64 {
		t.Fatalf("digest = %q", sum)
	}
	if filepath.Base(path) != "source-cccc2222.tar.gz" {
		t.Fatalf("saved as %s", filepath.Base(path))
	}
}
