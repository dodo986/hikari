package launch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hikari/internal/config"
	"hikari/internal/library"
)

const (
	WineNative       = "native"
	WinePlain        = "wine"
	WineProtonUmu    = "umu-proton"
	WineProtonDirect = "proton"
)

func storeToUmuStore(store string) string {
	switch store {
	case library.StoreEpic:
		return "egs"
	case library.StoreGOG:
		return "gog"
	case library.StoreAmazon:
		return "amazon"
	}
	return ""
}

func WrapperEnv(store, id string) map[string]string {
	env := map[string]string{
		"HIKARI_APP_NAME": id,
		"HIKARI_STORE":    store,
		"GAMEID":          "umu-0",
	}
	if s := storeToUmuStore(store); s != "" {
		env["STORE"] = s
	}
	return env
}

func SetupEnvVars(s config.GameSettings, installPath string, base map[string]string) {
	if s.NvidiaPrime {
		base["DRI_PRIME"] = "1"
		base["__NV_PRIME_RENDER_OFFLOAD"] = "1"
		base["__GLX_VENDOR_LIBRARY_NAME"] = "nvidia"
	}
	if installPath != "" {
		base["STEAM_COMPAT_INSTALL_PATH"] = installPath
	}
	for _, e := range s.Environment {
		if e.Key != "" {
			base[e.Key] = strings.Trim(e.Value, `"'`)
		}
	}
	if _, ok := base["LD_PRELOAD"]; !ok {
		base["LD_PRELOAD"] = ""
	}
}

// MergeDLLOverrides folds mod-required Wine DLL overrides into
// WINEDLLOVERRIDES, keeping whatever the game's own settings already asked for.
// Wine reads "name=order" pairs separated by semicolons, and the later entry
// wins, so the ones mods need go last.
func MergeDLLOverrides(env map[string]string, overrides map[string]string) {
	if len(overrides) == 0 {
		return
	}
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)

	seen := map[string]bool{}
	parts := make([]string, 0, len(overrides)+1)
	for _, part := range strings.Split(env["WINEDLLOVERRIDES"], ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, _, _ := strings.Cut(part, "=")
		if _, ok := overrides[strings.ToLower(name)]; ok {
			continue // the mod's value replaces this one
		}
		seen[strings.ToLower(name)] = true
		parts = append(parts, part)
	}
	for _, name := range names {
		if seen[name] {
			continue
		}
		parts = append(parts, name+"="+overrides[name])
	}
	env["WINEDLLOVERRIDES"] = strings.Join(parts, ";")
}

func SetupWineEnvVars(s config.GameSettings, wineType string, gameID string, home string) (map[string]string, error) {
	env := map[string]string{}
	switch wineType {
	case WinePlain:
		env["WINEPREFIX"] = s.WinePrefix
		const wmb = "winemenubuilder.exe=d"
		var userOverride string
		for _, e := range s.Environment {
			if strings.EqualFold(e.Key, "winedlloverrides") {
				userOverride = e.Value
			}
		}
		if userOverride != "" {
			env["WINEDLLOVERRIDES"] = userOverride + ";" + wmb
		} else {
			env["WINEDLLOVERRIDES"] = wmb
		}
		if s.EnableEsync {
			env["WINEESYNC"] = "1"
		}
		if s.EnableFsync {
			env["WINEFSYNC"] = "1"
		}
		if s.AutoInstallDxvkNvapi {
			env["DXVK_ENABLE_NVAPI"] = "1"
			env["DXVK_NVAPI_ALLOW_OTHER_DRIVERS"] = "1"
		}
	case WineProtonUmu, WineProtonDirect:
		env["WINEPREFIX"] = s.WinePrefix
		env["STEAM_COMPAT_DATA_PATH"] = s.WinePrefix
		env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(home, ".steam", "steam")
		if s.WineVersion.Bin != "" {
			env["PROTONPATH"] = filepath.Dir(s.WineVersion.Bin)
		}
		env["STEAM_COMPAT_APP_ID"] = "0"
		env["SteamAppId"] = "0"
		env["SteamGameId"] = "hikari-" + gameID
		env["PROTON_LOG_DIR"] = home
		if !s.EnableEsync {
			env["PROTON_NO_ESYNC"] = "1"
		}
		if !s.EnableFsync {
			env["PROTON_NO_FSYNC"] = "1"
		}
		if s.AutoInstallDxvkNvapi {
			env["PROTON_ENABLE_NVAPI"] = "1"
			env["DXVK_NVAPI_ALLOW_OTHER_DRIVERS"] = "1"
		} else {
			env["PROTON_DISABLE_NVAPI"] = "1"
		}
		if s.EnableWineWayland {
			env["PROTON_ENABLE_WAYLAND"] = "1"
			if s.EnableHDR {
				env["PROTON_ENABLE_HDR"] = "1"
			}
		}
		if s.EnableWoW64 {
			env["PROTON_USE_WOW64"] = "1"
		}
	default:
		return nil, fmt.Errorf("unsupported wine type %q", wineType)
	}

	if s.ShowFPS {
		env["DXVK_HUD"] = "fps"
	}
	if s.EnableFSR {
		env["WINE_FULLSCREEN_FSR"] = "1"
		strength := s.MaxSharpness
		if strength == 0 {
			strength = 2
		}
		env["WINE_FULLSCREEN_FSR_STRENGTH"] = strconv.Itoa(strength)
	} else {
		env["WINE_FULLSCREEN_FSR"] = "0"
	}
	return env, nil
}
