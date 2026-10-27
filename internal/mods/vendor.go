package mods

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"hikari/internal/mods/backend"
)

const backendDir = "backend"

// backendRoot is where the embedded Amethyst tree is written.
func backendRoot(root string) string { return filepath.Join(root, backendDir) }

// materializeBackend writes the embedded backend under root, and deletes files
// the snapshot no longer has. The stamp makes a second start a single stat.
func materializeBackend(root string) error {
	dst := backendRoot(root)
	stamp := filepath.Join(dst, "upstream.commit")
	if current, err := os.ReadFile(stamp); err == nil &&
		string(current) == backend.Upstream()+"\n" {
		return nil
	}
	wanted := map[string]bool{}
	err := fs.WalkDir(backend.Files(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, path)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := backend.Files().ReadFile(path)
		if err != nil {
			return err
		}
		wanted[path] = true
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		return fmt.Errorf("cannot write Amethyst's backend into %s: %w", dst, err)
	}
	if err := pruneBackend(dst, wanted); err != nil {
		return err
	}
	return os.WriteFile(stamp, []byte(backend.Upstream()+"\n"), 0o644)
}

// pruneBackend removes files an older snapshot left behind, so an upgrade never
// runs a mix of two Amethyst revisions.
func pruneBackend(dst string, wanted map[string]bool) error {
	var stale []string
	err := filepath.WalkDir(dst, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		if !wanted[filepath.ToSlash(rel)] {
			stale = append(stale, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}
