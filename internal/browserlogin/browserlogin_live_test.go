package browserlogin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBrowserSessionReadsCode(t *testing.T) {
	if os.Getenv("HIKARI_BROWSER_TEST") == "" {
		t.Skip("set HIKARI_BROWSER_TEST=1 to open a real browser window")
	}
	if _, ok := Available(); !ok {
		t.Skip("no Chromium-family browser installed")
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no graphical session")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/done?code=browser-test-code", http.StatusFound)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sess, err := Open(ctx, "epic", srv.URL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()

	code, err := sess.WaitCode(ctx, "epic")
	if err != nil {
		t.Fatalf("WaitCode: %v", err)
	}
	if code != "browser-test-code" {
		t.Fatalf("code = %q, want %q", code, "browser-test-code")
	}
}
