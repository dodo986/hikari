package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var assets embed.FS

// assetHandler serves the built bundle and injects the daemon endpoint and the
// palette into index.html, which is the only way a page can learn the token.
func assetHandler() http.Handler {
	root, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" || clean == "." {
			clean = "index.html"
		}
		if clean != "index.html" {
			files.ServeHTTP(w, r)
			return
		}
		page, err := fs.ReadFile(root, "index.html")
		if err != nil {
			http.Error(w, "the ui bundle has no index.html", http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		_, _ = w.Write(inject(page, bootstrap()))
	})
}

// bootstrap is what the page needs before its first render. A missing daemon is
// not fatal: the page shows its offline screen and retries.
func bootstrap() []byte {
	payload := map[string]any{}
	if endpoint, err := readEndpoint(); err == nil {
		payload["url"] = endpoint.URL
		payload["token"] = endpoint.Token
		payload["pid"] = endpoint.PID
	}
	payload["palette"] = readPalette()
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	script := "<script>window.__HIKARI_DAEMON__=" + string(encoded) + ";"
	// The link arrives even when the daemon is down, so it lives beside the
	// endpoint rather than inside it.
	if link := pendingLink(); link != "" {
		quoted, err := json.Marshal(link)
		if err == nil {
			script += "window.__HIKARI_LINK__=" + string(quoted) + ";"
		}
	}
	return []byte(script + "</script>")
}

func inject(page, script []byte) []byte {
	if len(script) == 0 {
		return page
	}
	marker := []byte("</body>")
	if i := bytes.LastIndex(page, marker); i >= 0 {
		out := make([]byte, 0, len(page)+len(script))
		out = append(out, page[:i]...)
		out = append(out, script...)
		return append(out, page[i:]...)
	}
	return append(page, script...)
}
