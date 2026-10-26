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

const stopTimeout = 30 * time.Second

func StopPrefix(ctx context.Context, r *engine.Runner, eng *engine.Manager, settings config.GameSettings) error {
	if settings.WinePrefix == "" {
		return fmt.Errorf("no wine prefix configured for this game")
	}

	if _, err := os.Stat(settings.WinePrefix); err != nil {
		return fmt.Errorf("no wine prefix at %s", settings.WinePrefix)
	}
	env := map[string]string{"WINEPREFIX": settings.WinePrefix}

	var argv []string
	if settings.WineVersion.Type == "proton" {

		if settings.WineVersion.Bin == "" {
			return fmt.Errorf("proton prefix but no proton binary set; set wine_version.bin")
		}
		argv = []string{settings.WineVersion.Bin, "run", "wineboot", "-k"}
		env["STEAM_COMPAT_DATA_PATH"] = settings.WinePrefix
		env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(userHomeDir(), ".steam", "steam")
		if dir := filepath.Dir(settings.WineVersion.Bin); dir != "" {
			env["PROTONPATH"] = dir
		}
	} else {
		bin := settings.WineVersion.Bin
		if bin == "" {
			var err error
			bin, err = eng.Require(engine.Wine)
			if err != nil {
				return err
			}
		}
		wineserver := filepath.Join(filepath.Dir(bin), "wineserver")
		argv = []string{wineserver, "-k"}
	}

	runCtx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()
	res := r.Run(runCtx, argv[0], argv[1:], engine.RunOptions{Env: env})
	if res.Err != nil {
		return fmt.Errorf("wine shutdown failed (%s): %s", filepath.Base(argv[0]), common.FirstLines(res.Stderr, 3))
	}
	return nil
}
