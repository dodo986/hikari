package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Endpoint mirrors what hikarid writes to $XDG_RUNTIME_DIR/hikari/ws.json. The
// host reads the same file the dev server does; it is rewritten on every daemon
// start, so a token never outlives the daemon that issued it.
type Endpoint struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

func runtimePath(name string) string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "hikari", name)
}

func readEndpoint() (map[string]any, error) {
	b, err := os.ReadFile(runtimePath("ws.json"))
	if err != nil {
		// The socket predates the websocket API: a daemon is up but cannot serve
		// this window, which is a different problem from no daemon at all.
		if _, sockErr := os.Stat(runtimePath("hikari.sock")); sockErr == nil {
			return nil, fmt.Errorf("the running hikarid is older than this frontend, rebuild it")
		}
		return nil, fmt.Errorf("hikarid is not running")
	}
	var endpoint Endpoint
	if err := json.Unmarshal(b, &endpoint); err != nil || endpoint.URL == "" {
		return nil, fmt.Errorf("the daemon endpoint file is unreadable")
	}
	return map[string]any{"url": endpoint.URL, "token": endpoint.Token, "pid": endpoint.PID}, nil
}
