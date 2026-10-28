package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"hikari/internal/api"
	"hikari/internal/atomicfile"
	"hikari/internal/config"
)

// Endpoint is what a local client needs to reach the daemon over websocket.
type Endpoint struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

// startWS binds loopback only and publishes the address plus a token that is
// regenerated on every start, so a stale page cannot reconnect after a restart.
func startWS(ctx context.Context, log *slog.Logger, srv *api.Server, artRoot string) (*Endpoint, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cannot bind the local websocket: %w", err)
	}
	token, err := newToken()
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	ep := &Endpoint{
		URL:   "ws://" + ln.Addr().String() + "/rpc",
		Token: token,
		PID:   os.Getpid(),
	}
	if err := writeEndpoint(ep); err != nil {
		_ = ln.Close()
		return nil, err
	}
	cfg := api.WSConfig{Token: token, Origins: wsOrigins(), Assets: artAssets(artRoot)}
	go func() {
		if err := srv.ServeWS(ctx, ln, cfg); err != nil {
			log.Error("websocket endpoint stopped", "err", err)
		}
	}()
	return ep, nil
}

// artAssets lets the browser load cached artwork over the same authorised
// listener, since a webview cannot read file:// paths.
func artAssets(root string) http.Handler {
	if root == "" {
		return nil
	}
	files := http.StripPrefix("/art/", http.FileServer(http.Dir(root)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/art/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("cache-control", "private, max-age=86400")
		files.ServeHTTP(w, r)
	})
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("cannot generate the websocket token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// wsOrigins allows the Wails window and the Vite dev server. Anything else is
// rejected during the handshake.
func wsOrigins() []string {
	if extra := os.Getenv("HIKARI_WS_ORIGINS"); extra != "" {
		out := []string{}
		for _, o := range strings.Split(extra, ",") {
			if o = strings.TrimSpace(o); o != "" {
				out = append(out, o)
			}
		}
		return out
	}
	return []string{"wails://wails", "http://localhost:5173", "http://127.0.0.1:5173"}
}

func writeEndpoint(ep *Endpoint) error {
	path := config.WSEndpointPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(path), err)
	}
	b, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteMode(path, append(b, '\n'), 0o600)
}

func removeEndpoint() {
	_ = os.Remove(config.WSEndpointPath())
}
