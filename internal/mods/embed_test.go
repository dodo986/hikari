package mods

import (
	"io/fs"
	"strings"
	"testing"

	"hikari/internal/mods/backend"
)

// The embedded tree is written into the user's data directory, so build output
// must never travel inside it. `all:native` used to embed the crate's cargo
// target directory whenever the extension had been built: hikarid grew by
// hundreds of megabytes and `hikari mods setup` copied the same artifacts into
// the user's data dir.
func TestEmbeddedBackendHasNoBuildOutput(t *testing.T) {
	skip := map[string]bool{"target": true, ".git": true, "__pycache__": true}
	err := fs.WalkDir(backend.Files(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skip[entry.Name()] {
				t.Errorf("the embedded backend contains build output: %s", path)
				return fs.SkipDir
			}
			return nil
		}
		for _, suffix := range []string{".rlib", ".rmeta", ".o", ".d", ".pyc", ".so"} {
			if strings.HasSuffix(path, suffix) {
				t.Errorf("the embedded backend contains a build artifact: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
