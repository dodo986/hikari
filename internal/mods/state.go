package mods

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// stateEnv points the backend at Hikari's own directories. Without it the
// backend would read and write Amethyst's config and staging folders.
func stateEnv(root string) map[string]string {
	return map[string]string{
		"XDG_CONFIG_HOME":          filepath.Join(root, "xdg"),
		"MOD_MANAGER_PROFILES_DIR": filepath.Join(root, "games"),
	}
}

// migrateState brings an existing Amethyst install's config and staged mods into
// Hikari's data directory. Hardlinks keep the first start instant and leave the
// originals untouched; the game folders themselves are never moved.
func migrateState(root, home string, log *slog.Logger) {
	if home == "" {
		return
	}
	env := stateEnv(root)
	oldConfig := filepath.Join(home, ".config", "AmethystModManager")
	oldStaging := filepath.Join(home, "Games", "Amethyst")
	newConfig := filepath.Join(env["XDG_CONFIG_HOME"], "AmethystModManager")
	newStaging := env["MOD_MANAGER_PROFILES_DIR"]
	for _, move := range [][2]string{{oldConfig, newConfig}, {oldStaging, newStaging}} {
		copied, err := copyTree(move[0], move[1])
		if err != nil {
			if log != nil {
				log.Warn("cannot migrate Amethyst state", "from", move[0], "err", err)
			}
			continue
		}
		if copied && log != nil {
			log.Info("migrated Amethyst state", "from", move[0], "to", move[1])
		}
	}
	if err := rewriteStaging(filepath.Join(newConfig, "games"), oldStaging, newStaging); err != nil && log != nil {
		log.Warn("cannot repoint migrated staging", "err", err)
	}
}

// copyTree links src into dst, skipping work when dst already exists. It reports
// whether anything was copied.
func copyTree(src, dst string) (bool, error) {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return false, nil
	}
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	}
	err = filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if err := os.Link(path, target); err == nil {
			return nil
		}
		return copyFile(path, target)
	})
	return err == nil, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// rewriteStaging repoints a migrated paths.json at the staging copy, so the
// backend keeps finding profiles and mods after the move.
func rewriteStaging(gamesDir, oldRoot, newRoot string) error {
	entries, err := os.ReadDir(gamesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		file := filepath.Join(gamesDir, entry.Name(), "paths.json")
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			continue
		}
		staging, _ := data["staging_path"].(string)
		if !underDir(staging, oldRoot) {
			continue
		}
		data["staging_path"] = filepath.Join(newRoot, strings.TrimPrefix(filepath.ToSlash(staging), filepath.ToSlash(oldRoot)+"/"))
		updated, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(file, append(updated, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func underDir(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	return path == dir || strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(dir)+"/")
}
