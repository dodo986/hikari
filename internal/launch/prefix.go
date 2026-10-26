package launch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/providers/common"
)

const prefixInitTimeout = 10 * time.Minute

func prefixInitArgv(settings config.GameSettings, eng *engine.Manager, wineType string) ([]string, error) {
	switch wineType {
	case WinePlain:
		bin := settings.WineVersion.Bin
		if bin == "" {
			var err error
			bin, err = eng.Require(engine.Wine)
			if err != nil {
				return nil, err
			}
		}
		return []string{bin, "wineboot", "--init"}, nil
	case WineProtonUmu:
		bin, err := eng.Require(engine.Umu)
		if err != nil {
			return nil, err
		}
		return []string{bin, "createprefix"}, nil
	case WineProtonDirect:
		if settings.WineVersion.Bin == "" {
			return nil, fmt.Errorf("proton selected but no proton binary set; set wine_version.bin")
		}
		return []string{settings.WineVersion.Bin, "run", "wineboot", "--init"}, nil
	}
	return nil, fmt.Errorf("cannot prepare a prefix for wine type %q", wineType)
}

// A wineboot killed mid-run leaves no syswow64 and PE32 then fails with c0000135.
func prefixHasArch(prefix, wineBin, wineType string) bool {
	if wineType != WinePlain {
		return true
	}
	if !wineHas32Bit(wineBin) {
		return true
	}
	return isDir(filepath.Join(prefix, "drive_c", "windows", "syswow64"))
}

func wineHas32Bit(wineBin string) bool {
	if wineBin == "" {
		return false
	}
	lib, lib32 := WineLibs(wineBin)
	for _, dir := range []string{lib, lib32} {
		if dir != "" && isDir(filepath.Join(dir, "wine", "i386-windows")) {
			return true
		}
	}
	return false
}

func EnsurePrefix(ctx context.Context, r *engine.Runner, eng *engine.Manager, settings config.GameSettings, wineType string, env map[string]string) error {
	if wineType == WineNative {
		return nil
	}
	prefix := settings.WinePrefix
	if prefix == "" {
		return fmt.Errorf("no wine prefix configured; set wine_prefix in the game settings")
	}
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return fmt.Errorf("cannot create wine prefix %s: %w", prefix, err)
	}
	reg := filepath.Join(prefix, "system.reg")
	if wineType == WineProtonUmu || wineType == WineProtonDirect {
		reg = filepath.Join(prefix, "pfx", "system.reg")
	}
	argv, err := prefixInitArgv(settings, eng, wineType)
	if err != nil {
		return err
	}
	if _, err := os.Stat(reg); err == nil && prefixHasArch(prefix, argv[0], wineType) {
		return nil
	}

	runCtx, cancel := context.WithTimeout(ctx, prefixInitTimeout)
	defer cancel()
	res := r.Run(runCtx, argv[0], argv[1:], engine.RunOptions{Env: env})
	if res.Err != nil {
		return fmt.Errorf("prefix setup failed (%s): %s", argv[len(argv)-1], common.FirstLines(res.Stderr, 3))
	}
	return nil
}
