package launch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hikari/internal/config"
	"hikari/internal/dxvk"
	"hikari/internal/engine"
	"hikari/internal/providers/common"
	"hikari/internal/runtimes"
)

type ToolInstaller interface {
	Ensure(ctx context.Context, tool dxvk.Tool) (version, tree string, err error)
	Cached(tool dxvk.Tool, version string) (string, bool)
	CheckDue(tool dxvk.Tool, maxAge time.Duration) bool
	InstallInto(ctx context.Context, prefix string, tool dxvk.Tool, version, tree string, reg dxvk.RegFunc) ([]string, error)
}

const toolCheckInterval = 24 * time.Hour

func PrepareExtras(ctx context.Context, r *engine.Runner, eng *engine.Manager, tools ToolInstaller, log *slog.Logger, s config.GameSettings, wineType string, env map[string]string) {
	if tools == nil || wineType != WinePlain || s.WinePrefix == "" {
		return
	}
	wanted := enabledTools(s)
	if len(wanted) == 0 {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	wineBin := s.WineVersion.Bin
	if wineBin == "" {
		var err error
		wineBin, err = eng.Require(engine.Wine)
		if err != nil {
			log.Warn("skipping DXVK tools: no wine binary", "err", err)
			return
		}
	}
	for _, tool := range wanted {
		have := currentVersion(s.WinePrefix, tool)
		version, tree, err := resolveTool(ctx, tools, tool, s.WinePrefix)
		if err != nil {
			log.Warn("tool install skipped", "tool", tool, "err", err)
			continue
		}
		if have != "" && have == version {
			log.Debug("tool already in prefix", "tool", tool, "version", version)
			continue
		}
		reg := func(ctx context.Context, dll string) error {
			res := r.Run(ctx, wineBin, []string{
				"reg", "add", "HKEY_CURRENT_USER\\Software\\Wine\\DllOverrides",
				"/v", dll, "/d", "native,builtin", "/f",
			}, engine.RunOptions{Env: env})
			if res.Err != nil {
				return fmt.Errorf("%s", common.FirstLines(res.Stderr, 2))
			}
			return nil
		}
		installed, err := tools.InstallInto(ctx, s.WinePrefix, tool, version, tree, reg)
		if err != nil {
			log.Warn("tool install failed", "tool", tool, "version", version, "err", err)
			continue
		}
		log.Info("installed tool into prefix", "tool", tool, "version", version, "dlls", len(installed))
	}
}

type RuntimeInstaller interface {
	Installed(name runtimes.Name) (string, bool)
	Ensure(ctx context.Context, name runtimes.Name) (string, error)
}

func PrepareRuntimes(ctx context.Context, inst RuntimeInstaller, log *slog.Logger, s config.GameSettings, wineType string) {
	if inst == nil || wineType == WineNative {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	wanted := []struct {
		name runtimes.Name
		on   bool
	}{
		{runtimes.EAC, s.EacRuntime},
		{runtimes.BattlEye, s.BattlEyeRuntime},
	}
	for _, w := range wanted {
		if !w.on {
			continue
		}
		if _, ok := inst.Installed(w.name); ok {
			continue
		}
		dir, err := inst.Ensure(ctx, w.name)
		if err != nil {
			log.Warn("runtime install skipped", "runtime", w.name, "err", err)
			continue
		}
		log.Info("installed runtime", "runtime", w.name, "dir", dir)
	}
}

func enabledTools(s config.GameSettings) []dxvk.Tool {
	var out []dxvk.Tool
	if s.AutoInstallDxvk {
		out = append(out, dxvk.DXVK)
	}
	if s.AutoInstallVkd3d {
		out = append(out, dxvk.VKD3D)
	}
	if s.AutoInstallDxvkNvapi {
		out = append(out, dxvk.NVAPI)
	}
	return out
}

func resolveTool(ctx context.Context, tools ToolInstaller, tool dxvk.Tool, prefix string) (string, string, error) {
	if version := currentVersion(prefix, tool); version != "" && !tools.CheckDue(tool, toolCheckInterval) {
		if tree, ok := tools.Cached(tool, version); ok {
			return version, tree, nil
		}
	}
	return tools.Ensure(ctx, tool)
}

func currentVersion(prefix string, tool dxvk.Tool) string {
	b, err := os.ReadFile(filepath.Join(prefix, "current_"+string(tool)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
