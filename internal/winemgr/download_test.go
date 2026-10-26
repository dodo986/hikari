package winemgr

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

type failingBody struct{ sent bool }

func (f *failingBody) Read(p []byte) (int, error) {
	if !f.sent {
		f.sent = true
		n := copy(p, "partial")
		return n, nil
	}
	return 0, errors.New("connection reset")
}

func (f *failingBody) Close() error { return nil }

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: &failingBody{}, Header: http.Header{}}, nil
}

func TestInterruptedDownloadIsRejected(t *testing.T) {
	m := NewForTest(t.TempDir(), t.TempDir(), nil)
	m.client = &http.Client{Transport: failingTransport{}}
	if err := os.MkdirAll(m.protonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rel := Release{Name: "GE-Proton-test", download: "https://example.invalid/x.tar.xz", Size: 1024}
	_, err := m.downloadVerified(context.Background(), rel, "", nil)
	if err == nil {
		t.Fatal("a truncated download was accepted")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(m.protonDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".download-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
