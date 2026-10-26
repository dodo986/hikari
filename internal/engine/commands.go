package engine

import (
	"fmt"
	"os"
	"strings"

	"hikari/internal/config"
)

func LegendaryList() []string { return []string{"list", "--third-party"} }

func LegendaryInfo(app, platform string) []string {
	return []string{"info", app, "--json", "--platform", platform}
}

func LegendaryInstallArgs(app, basePath, platform string, maxWorkers int, sdlTags []string) []string {
	args := []string{"install", app, "--platform", platform, "--base-path", basePath, "--skip-dlcs", "-y"}
	if len(sdlTags) > 0 {
		args = append(args, "--install-tag=")
		for _, tag := range sdlTags {
			args = append(args, "--install-tag", tag)
		}
	} else {
		args = append(args, "--skip-sdl")
	}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func LegendaryUpdateArgs(app string, maxWorkers int) []string {
	args := []string{"update", app, "-y", "--skip-sdl"}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func LegendaryRepairArgs(app string, maxWorkers int) []string {
	args := []string{"repair", app, "-y", "--skip-sdl"}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func LegendarySyncSavesArgs(app, savePath, direction string) []string {
	dir := "--download"
	if direction == "upload" {
		dir = "--upload"
	}
	return []string{"sync-saves", app, dir, "--save-path", savePath, "-y"}
}

func LegendaryUninstallArgs(app string, keepFiles bool) []string {
	args := []string{"uninstall", app, "-y"}
	if keepFiles {
		args = append(args, "--keep-files")
	}
	return args
}

func LegendaryLaunchArgs(app string, extraArgs []string, wrapper, wineBin, overrideExe, language string, offline bool) []string {
	args := []string{"launch", app}
	args = append(args, extraArgs...)
	switch {
	case wineBin != "" && wrapper != "":
		args = append(args, "--wine", wineBin, "--wrapper", wrapper)
	case wrapper != "":
		args = append(args, "--no-wine", "--wrapper", wrapper)
	case wineBin != "":
		args = append(args, "--wine", wineBin)
	}
	if overrideExe != "" {
		args = append(args, "--override-exe", overrideExe)
	}
	if language != "" {
		args = append(args, "--language", language)
	}
	if offline {
		args = append(args, "--offline")
	}
	return args
}

func LegendaryAuthCode(code string) []string { return []string{"auth", "--code", code} }
func LegendaryAuthDelete() []string          { return []string{"auth", "--delete"} }

func GogdlAuthHeader(authConfigPath string) []string {
	return []string{"--auth-config-path", authConfigPath}
}

func GogdlDownloadArgs(id, platform, path, supportDir, language string, dlcs []string, maxWorkers int) []string {
	args := []string{"download", id, "--platform", platform, "--path", path, "--support", supportDir}
	if len(dlcs) > 0 {
		args = append(args, "--with-dlcs", "--dlcs", strings.Join(dlcs, ","))
	} else {
		args = append(args, "--skip-dlcs")
	}
	if language != "" {
		args = append(args, "--lang", language)
	}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func GogdlUpdateArgs(id, platform, path, supportDir, language, buildID string, maxWorkers int) []string {
	args := []string{"update", id, "--platform", platform, "--path", path, "--support", supportDir, "--skip-dlcs"}
	if language != "" {
		args = append(args, "--lang", language)
	}
	if buildID != "" {
		args = append(args, "-b="+buildID)
	}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func GogdlRepairArgs(id, platform, path, supportDir, language, buildID string, maxWorkers int) []string {
	args := []string{"repair", id, "--platform", platform, "--path", path, "--support", supportDir, "--skip-dlcs"}
	if language != "" {
		args = append(args, "--lang", language)
	}
	if buildID != "" {
		args = append(args, "-b="+buildID)
	}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return args
}

func GogdlLaunchArgs(installPath, id, platform string, extraArgs []string, wrapper, wineBin, overrideExe string) []string {
	args := []string{"launch", installPath}
	if overrideExe != "" {
		args = append(args, "--override-exe", overrideExe)
	}
	args = append(args, id)
	switch {
	case wineBin != "" && wrapper != "":
		args = append(args, "--wine", wineBin, "--wrapper", wrapper)
	case wineBin != "":
		args = append(args, "--wine", wineBin)
	case wrapper != "":
		args = append(args, "--no-wine", "--wrapper", wrapper)
	}
	args = append(args, "--platform", platform)
	args = append(args, extraArgs...)
	return args
}

func GogdlAuthCode(code string) []string { return []string{"auth", "--code", code} }

func GogdlAuthRefresh() []string { return []string{"auth"} }

func NileLibrarySync() []string { return []string{"library", "sync"} }
func NileInstallArgs(id, basePath string, maxWorkers int) []string {
	args := []string{"install", "--base-path", basePath}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return append(args, id)
}
func NileUpdateArgs(id string, maxWorkers int) []string {
	args := []string{"update"}
	if maxWorkers > 0 {
		args = append(args, "--max-workers", fmt.Sprint(maxWorkers))
	}
	return append(args, id)
}
func NileUninstallArgs(id string) []string { return []string{"uninstall", id} }

func NileLaunchArgs(id, wrapper, wineBin, winePrefix, overrideExe string, extraArgs []string) []string {
	args := []string{"launch"}
	if overrideExe != "" {
		args = append(args, "--override-exe", overrideExe)
	}
	switch {
	case wineBin != "" && wrapper != "":
		args = append(args, "--wine", wineBin, "--wrapper", wrapper)
	case wrapper != "":
		args = append(args, "--no-wine", "--wrapper", wrapper)
	case wineBin != "":
		args = append(args, "--wine", wineBin)
	}
	if winePrefix != "" {
		args = append(args, "--wine-prefix", winePrefix)
	}
	args = append(args, extraArgs...)
	return append(args, id)
}

func NileVerifyArgs(id, path string) []string {
	return []string{"verify", "--path", path, id}
}

func NileAuthLogin() []string { return []string{"auth", "--login", "--non-interactive"} }

func NileRegisterArgs(code, codeVerifier, serial, clientID string) []string {
	return []string{"register", "--code", code, "--code-verifier", codeVerifier, "--serial", serial, "--client-id", clientID}
}

func NileLogout() []string { return []string{"auth", "--logout"} }

func EnvFor(e Engine, paths config.Paths) map[string]string {
	switch e {
	case Legendary:
		return map[string]string{"LEGENDARY_CONFIG_PATH": paths.EngineEpic}
	case Gogdl:
		return map[string]string{"GOGDL_CONFIG_PATH": paths.EngineGOG}
	case Nile:
		return map[string]string{"NILE_CONFIG_PATH": paths.EngineAmazon}
	}
	return nil
}

func osEnviron() []string { return os.Environ() }

func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
