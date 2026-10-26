package launch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

func PrepareEOSOverlay(ctx context.Context, r *engine.Runner, eng *engine.Manager, log *slog.Logger, s config.GameSettings, game library.Game, overlayDir string, env map[string]string) {
	if !s.EnableEOSOverlay || s.WinePrefix == "" || game.Store != library.StoreEpic {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	bin, err := eng.Require(engine.Legendary)
	if err != nil {
		log.Warn("EOS overlay skipped", "err", err)
		return
	}
	info := r.Run(ctx, bin, []string{"eos-overlay", "info", "--prefix", s.WinePrefix}, engine.RunOptions{Env: env})
	if info.Err == nil && strings.Contains(info.Stdout+info.Stderr, "Overlay enabled: Yes") {
		return
	}
	if !overlayInstalled(overlayDir) {
		res := r.Run(ctx, bin, []string{"eos-overlay", "install", "--path", overlayDir, "-y"}, engine.RunOptions{Env: env})
		if res.Err != nil {
			log.Warn("EOS overlay install failed", "err", res.Err, "output", firstLine(res.Stderr))
			return
		}
	}
	res := r.Run(ctx, bin, []string{"eos-overlay", "enable", "--prefix", s.WinePrefix}, engine.RunOptions{Env: env})
	if res.Err != nil {
		log.Warn("EOS overlay enable failed", "err", res.Err, "output", firstLine(res.Stderr))
		return
	}
	log.Info("EOS overlay enabled", "prefix", s.WinePrefix)
}

func overlayInstalled(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func EOSOverlayDir(dataDir string) string {
	return filepath.Join(dataDir, "eos_overlay")
}
