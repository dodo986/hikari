package launch

import (
	"os"
	"path/filepath"
	"strings"

	"hikari/internal/config"
	"hikari/internal/library"
)

func DefaultPrefixDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/hikari-prefixes"
	}
	return filepath.Join(home, "Games", "Hikari", "Prefixes")
}

func GamePrefixDir(g library.Game) string {
	return filepath.Join(DefaultPrefixDir(), PrefixSlug(g))
}

func PrefixSlug(g library.Game) string {
	name := strings.TrimSpace(g.Title)
	if name == "" {
		name = g.ID
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '\t':
			if s := b.String(); s != "" && !strings.HasSuffix(s, "-") {
				b.WriteByte('-')
			}
		}
	}
	slug := strings.Trim(b.String(), "-._")
	if slug == "" {
		slug = g.ID
	}
	return slug
}

func FactorySettings(defaults config.DefaultsSettings) config.GameSettings {
	s := config.GameSettings{
		EnableEsync: true,
		EnableFsync: true,
		WineVersion: defaults.WineVersion,
	}
	s.WinePrefix = defaults.WinePrefix
	if s.WinePrefix == "" {
		s.WinePrefix = filepath.Join(DefaultPrefixDir(), "default")
	}
	if s.WineVersion.Type == "" {
		s.WineVersion.Type = "wine"
	}
	return s
}

func EffectiveSettings(g library.Game, defaults config.DefaultsSettings, entry *config.GameEntry, imported *config.GameSettings) config.GameSettings {
	s := FactorySettings(defaults)
	perGame := defaults.WinePrefix == ""
	if imported != nil {
		if imported.WinePrefix != "" {
			perGame = false
		}
		mergeSettings(&s, *imported, false)
	}
	if entry != nil {
		if entry.Settings.WinePrefix != "" {
			perGame = false
		}

		mergeSettings(&s, entry.Settings, entry.SettingsComplete)
	}
	if perGame {
		s.WinePrefix = GamePrefixDir(g)
	}
	return s
}

func DropDefaultPrefix(g library.Game, defaults config.DefaultsSettings, s *config.GameSettings) {
	if s == nil || s.WinePrefix == "" || defaults.WinePrefix != "" {
		return
	}
	if filepath.Clean(s.WinePrefix) == filepath.Clean(GamePrefixDir(g)) {
		s.WinePrefix = ""
	}
}

func mergeSettings(dst *config.GameSettings, src config.GameSettings, exactBools bool) {
	if src.WinePrefix != "" {
		dst.WinePrefix = src.WinePrefix
	}
	if src.WineVersion.Bin != "" || src.WineVersion.Type != "" {
		dst.WineVersion = src.WineVersion
	}
	if src.TargetExe != "" {
		dst.TargetExe = src.TargetExe
	}
	if src.LauncherArgs != "" {
		dst.LauncherArgs = src.LauncherArgs
	}
	if src.Language != "" {
		dst.Language = src.Language
	}
	if len(src.Environment) > 0 {
		dst.Environment = src.Environment
	}
	if len(src.Wrappers) > 0 {
		dst.Wrappers = src.Wrappers
	}
	if src.Gamescope != nil {
		dst.Gamescope = src.Gamescope
	}
	if src.MaxSharpness != 0 {
		dst.MaxSharpness = src.MaxSharpness
	}
	if src.BeforeScript != "" {
		dst.BeforeScript = src.BeforeScript
	}
	if src.AfterScript != "" {
		dst.AfterScript = src.AfterScript
	}
	if exactBools {
		dst.UseGameMode = src.UseGameMode
		dst.ShowMangoHud = src.ShowMangoHud
		dst.ShowFPS = src.ShowFPS
		dst.EnableEsync = src.EnableEsync
		dst.EnableFsync = src.EnableFsync
		dst.EnableFSR = src.EnableFSR
		dst.EnableWineWayland = src.EnableWineWayland
		dst.EnableHDR = src.EnableHDR
		dst.EnableWoW64 = src.EnableWoW64
		dst.AutoInstallDxvk = src.AutoInstallDxvk
		dst.AutoInstallVkd3d = src.AutoInstallVkd3d
		dst.AutoInstallDxvkNvapi = src.AutoInstallDxvkNvapi
		dst.EacRuntime = src.EacRuntime
		dst.BattlEyeRuntime = src.BattlEyeRuntime
		dst.EnableEOSOverlay = src.EnableEOSOverlay
		dst.NvidiaPrime = src.NvidiaPrime
		dst.OfflineMode = src.OfflineMode
		dst.PreferSystemLibs = src.PreferSystemLibs
		dst.DisableUMU = src.DisableUMU
		return
	}
	dst.UseGameMode = src.UseGameMode || dst.UseGameMode
	dst.ShowMangoHud = src.ShowMangoHud || dst.ShowMangoHud
	dst.ShowFPS = src.ShowFPS || dst.ShowFPS
	dst.EnableEsync = src.EnableEsync || dst.EnableEsync
	dst.EnableFsync = src.EnableFsync || dst.EnableFsync
	dst.EnableFSR = src.EnableFSR || dst.EnableFSR
	dst.EnableWineWayland = src.EnableWineWayland || dst.EnableWineWayland
	dst.EnableHDR = src.EnableHDR || dst.EnableHDR
	dst.EnableWoW64 = src.EnableWoW64 || dst.EnableWoW64
	dst.AutoInstallDxvk = src.AutoInstallDxvk || dst.AutoInstallDxvk
	dst.AutoInstallVkd3d = src.AutoInstallVkd3d || dst.AutoInstallVkd3d
	dst.AutoInstallDxvkNvapi = src.AutoInstallDxvkNvapi || dst.AutoInstallDxvkNvapi
	dst.EacRuntime = src.EacRuntime || dst.EacRuntime
	dst.BattlEyeRuntime = src.BattlEyeRuntime || dst.BattlEyeRuntime
	dst.EnableEOSOverlay = src.EnableEOSOverlay || dst.EnableEOSOverlay
	dst.NvidiaPrime = src.NvidiaPrime || dst.NvidiaPrime
	dst.OfflineMode = src.OfflineMode || dst.OfflineMode
	dst.PreferSystemLibs = src.PreferSystemLibs || dst.PreferSystemLibs
	dst.DisableUMU = src.DisableUMU || dst.DisableUMU
}
