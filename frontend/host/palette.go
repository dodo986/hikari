package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// paletteTick is how often the matugen file is re-read. A change repaints the
// window in place, the way the QML shell followed the same file.
const paletteTick = time.Second

func palettePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "ryoku", "colors.json")
}

// readPalette is the wallpaper's colour roles, or nil when Ryoku has not
// generated them and the compiled defaults should stand.
func readPalette() any {
	path := palettePath()
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var roles map[string]any
	if err := json.Unmarshal(b, &roles); err != nil {
		return nil
	}
	return roles
}

func paletteStamp() string {
	path := palettePath()
	if path == "" {
		return "none"
	}
	info, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	return fmt.Sprintf("%d-%d", info.ModTime().UnixNano(), info.Size())
}

// watchPalette emits the roles whenever the file changes, so a matugen run
// recolours an open window instead of requiring a restart.
func (h *Host) watchPalette() {
	last := paletteStamp()
	for range time.Tick(paletteTick) {
		stamp := paletteStamp()
		if stamp == last {
			continue
		}
		last = stamp
		roles := readPalette()
		if roles == nil || h.ctx == nil {
			continue
		}
		runtime.EventsEmit(h.ctx, "palette", roles)
	}
}
