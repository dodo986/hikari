package launch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

type Plan struct {
	Argv     []string                        `json:"argv"`
	Env      map[string]string               `json:"env"`
	Cwd      string                          `json:"cwd,omitempty"`
	Engine   string                          `json:"engine"`
	WineType string                          `json:"wine_type"`
	Prepare  func(ctx context.Context) error `json:"-"`
	Prefix   string                          `json:"-"`
	Sidecars []Sidecar                       `json:"sidecars,omitempty"`
}

type Sidecar struct {
	Name string            `json:"name"`
	Argv []string          `json:"argv"`
	Env  map[string]string `json:"env,omitempty"`
}

func (p *Plan) EnvDiff() string {
	keys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+p.Env[k])
	}
	return strings.Join(lines, "\n")
}

type Options struct {
	Args          []string
	UmuID         string
	AppIDOverride string
	// ModOverrides are Wine DLL overrides the installed mods need (proxy DLLs
	// such as version.dll or winmm.dll only load when forced to native).
	ModOverrides map[string]string
}

func Build(game library.Game, settings config.GameSettings, paths config.Paths, eng *engine.Manager, opts Options) (*Plan, error) {
	plan := &Plan{Engine: game.Runner}

	native := isNative(game)
	if native {
		plan.WineType = WineNative
	} else {
		wt, err := selectWine(settings, eng)
		if err != nil {
			return nil, err
		}
		plan.WineType = wt
		plan.Prefix = settings.WinePrefix
	}

	var wineBin string
	if !native && plan.WineType == WinePlain {
		wineBin = settings.WineVersion.Bin
		if wineBin == "" {
			var err error
			wineBin, err = eng.Require(engine.Wine)
			if err != nil {
				return nil, err
			}
		}
	}

	env := WrapperEnv(game.Store, game.ID)
	if opts.UmuID != "" {
		env["GAMEID"] = opts.UmuID
	}
	SetupEnvVars(settings, game.InstallPath, env)
	if !native {
		wineEnv, err := SetupWineEnvVars(settings, plan.WineType, game.ID, userHomeDir())
		if err != nil {
			return nil, err
		}
		for k, v := range wineEnv {
			env[k] = v
		}
		SetupWineLibEnv(settings, plan.WineType, wineBin, env)
		MergeDLLOverrides(env, opts.ModOverrides)
		if settings.EacRuntime {
			env["PROTON_EAC_RUNTIME"] = filepath.Join(paths.DataDir, "runtimes", "eac_runtime")
		}
		if settings.BattlEyeRuntime {
			env["PROTON_BATTLEYE_RUNTIME"] = filepath.Join(paths.DataDir, "runtimes", "battleye_runtime")
		}
	}

	if e, ok := engineForRunner(game.Runner); ok {
		for k, v := range engine.EnvFor(e, paths) {
			env[k] = v
		}
	}
	plan.Env = env

	var umuBin string
	if plan.WineType == WineProtonUmu {
		resolved, err := eng.Require(engine.Umu)
		if err != nil {
			return nil, err
		}
		umuBin = resolved
	}
	wrappers := Wrappers(settings, umuBin)
	wrapper := ShellJoin(wrappers)

	extra := append([]string{}, opts.Args...)
	if settings.LauncherArgs != "" {
		extra = append(extra, strings.Fields(settings.LauncherArgs)...)
	}
	overrideExe := settings.TargetExe

	if game.Runner == library.RunnerLocal {
		return buildLocal(game, settings, wrappers, wineBin, extra, plan), nil
	}

	e, ok := engineForRunner(game.Runner)
	if !ok {
		return nil, fmt.Errorf("no engine for runner %q", game.Runner)
	}
	bin, err := eng.Require(e)
	if err != nil {
		return nil, err
	}

	switch game.Runner {
	case library.RunnerLegendary:
		var wineFlag string
		if !native {
			switch plan.WineType {
			case WinePlain:
				wineFlag = wrapper
			case WineProtonUmu:
				wineFlag = wrapper
			case WineProtonDirect:
				wineFlag = ShellJoin(append(append([]string{}, wrappers...), settings.WineVersion.Bin, "waitforexitandrun"))
			}
		}
		launchID := game.ID
		if opts.AppIDOverride != "" {
			launchID = opts.AppIDOverride
		}
		plan.Argv = append([]string{bin},
			engine.LegendaryLaunchArgs(launchID, extra, wineFlag, wineBin, overrideExe, settings.Language, settings.OfflineMode)...)
	case library.RunnerGogdl:
		plan.Argv = []string{bin}
		plan.Argv = append(plan.Argv, engine.GogdlAuthHeader(filepath.Join(paths.EngineGOG, "auth.json"))...)
		var wineFlag string
		if !native {
			switch plan.WineType {
			case WinePlain:
				wineFlag = wrapper
			case WineProtonUmu:
				wineFlag = wrapper
			case WineProtonDirect:
				wineFlag = ShellJoin(append(append([]string{}, wrappers...), settings.WineVersion.Bin, "waitforexitandrun"))
			}
		}
		plan.Argv = append(plan.Argv, engine.GogdlLaunchArgs(game.InstallPath, game.ID, platformArg(game), extra, wineFlag, wineBin, overrideExe)...)
	case library.RunnerNile:
		plan.Argv = []string{bin}
		var wineFlag, winePrefix string
		if !native {
			switch plan.WineType {
			case WinePlain:
				wineFlag = wrapper
			case WineProtonUmu:
				wineFlag = wrapper
			case WineProtonDirect:
				wineFlag = ShellJoin(append(append([]string{}, wrappers...), settings.WineVersion.Bin, "waitforexitandrun"))
			}
			winePrefix = settings.WinePrefix
		}
		plan.Argv = append(plan.Argv, engine.NileLaunchArgs(game.ID, wineFlag, wineBin, winePrefix, overrideExe, extra)...)
	default:
		return nil, fmt.Errorf("no launch runner for store %q", game.Store)
	}
	return plan, nil
}

func buildLocal(game library.Game, settings config.GameSettings, wrappers []string, wineBin string, extra []string, plan *Plan) *Plan {
	exe := settings.TargetExe
	if exe == "" {
		exe = game.Executable
	}
	// A relative target ("bin/x64/Game.exe") is relative to the install folder,
	// and the game must start there: Wine and crash reporters drop their files
	// in the working directory.
	if exe != "" && !filepath.IsAbs(exe) && game.InstallPath != "" {
		exe = filepath.Join(game.InstallPath, exe)
	}
	if exe != "" {
		plan.Cwd = filepath.Dir(exe)
	}
	var argv []string
	argv = append(argv, wrappers...)
	switch plan.WineType {
	case WineNative, WineProtonUmu:
		argv = append(argv, exe)
	case WinePlain:
		argv = append(argv, wineBin, exe)
	case WineProtonDirect:
		argv = append(argv, settings.WineVersion.Bin, "waitforexitandrun", exe)
	}
	if e, ok := localArgs(game); ok {
		argv = append(argv, e...)
	}
	argv = append(argv, extra...)
	plan.Argv = argv
	plan.Engine = "local"
	return plan
}

func localArgs(game library.Game) ([]string, bool) {
	raw := game.Metadata["args"]
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	return strings.Fields(raw), true
}

func selectWine(settings config.GameSettings, eng *engine.Manager) (string, error) {
	wt := settings.WineVersion.Type
	switch wt {
	case "proton":
		if !settings.DisableUMU {
			if _, err := eng.Require(engine.Umu); err == nil {
				return WineProtonUmu, nil
			}
		}
		if settings.WineVersion.Bin == "" {
			return "", fmt.Errorf("proton selected but no proton binary set; set wine_version.bin or install umu-launcher")
		}
		return WineProtonDirect, nil
	case "crossover", "toolkit":
		return "", fmt.Errorf("wine type %q is not supported on Linux yet", wt)
	default:
		if settings.WineVersion.Bin == "" {
			if _, err := eng.Require(engine.Wine); err != nil {
				return "", fmt.Errorf("no wine configured: %s", err)
			}
		}
		return WinePlain, nil
	}
}

func isNative(game library.Game) bool {
	return strings.EqualFold(game.Platform, "Linux")
}

func engineForRunner(runner string) (engine.Engine, bool) {
	switch runner {
	case library.RunnerLegendary:
		return engine.Legendary, true
	case library.RunnerGogdl:
		return engine.Gogdl, true
	case library.RunnerNile:
		return engine.Nile, true
	}
	return "", false
}

func platformArg(game library.Game) string {
	if game.Platform == "" {
		return "windows"
	}
	return strings.ToLower(game.Platform)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/root"
	}
	return home
}
