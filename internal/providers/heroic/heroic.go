package heroic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"hikari/internal/config"
	"hikari/internal/library"
)

type Importer struct {
	root string

	mu    sync.Mutex
	cache map[string]library.Game
}

func New(root string) *Importer {
	return &Importer{root: root, cache: map[string]library.Game{}}
}

func (i *Importer) Store() string { return library.StoreHeroic }

type HeroicGameInfo struct {
	AppName     string `json:"app_name"`
	Runner      string `json:"runner"`
	Title       string `json:"title"`
	Developer   string `json:"developer"`
	IsInstalled bool   `json:"is_installed"`
	Install     struct {
		InstallPath string `json:"install_path"`
		Platform    string `json:"platform"`
		Version     string `json:"version"`
		Executable  string `json:"executable"`
		InstallSize string `json:"install_size"`
		IsDLC       bool   `json:"is_dlc"`
	} `json:"install"`
	ArtCover      string `json:"art_cover"`
	ArtSquare     string `json:"art_square"`
	ArtLogo       string `json:"art_logo"`
	ArtIcon       string `json:"art_icon"`
	CloudSave     bool   `json:"cloud_save_enabled"`
	CanRunOffline bool   `json:"canRunOffline"`
	Namespace     string `json:"namespace"`
	StoreURL      string `json:"store_url"`
	IsLinuxNative bool   `json:"is_linux_native"`
	IsMacNative   bool   `json:"is_mac_native"`
}

var runnerToStore = map[string]string{
	"legendary": library.StoreEpic,
	"gog":       library.StoreGOG,
	"nile":      library.StoreAmazon,
}

func (i *Importer) List(ctx context.Context) ([]library.Game, error) {
	games := map[string]library.Game{}

	for cacheFile, runner := range map[string]string{
		"legendary_library.json": "legendary",
		"gog_library.json":       "gog",
		"nile_library.json":      "nile",
	} {
		entries := readHeroicGameInfos(filepath.Join(i.root, "store_cache", cacheFile))
		for _, e := range entries {
			g, ok := i.convert(e, runner)
			if !ok {
				continue
			}
			games[g.Key()] = g
		}
	}

	i.mergeLegendaryInstalled(games)
	i.mergeGOGInstalled(games)
	i.mergeNileInstalled(games)

	out := make([]library.Game, 0, len(games))
	for _, g := range games {
		out = append(out, g)
	}
	i.mu.Lock()
	i.cache = games
	i.mu.Unlock()
	return out, nil
}

func (i *Importer) convert(e HeroicGameInfo, runner string) (library.Game, bool) {
	store, ok := runnerToStore[runner]
	if !ok || e.AppName == "" || e.IsDLCCheck() {
		return library.Game{}, false
	}
	g := library.Game{
		Store:         store,
		ID:            e.AppName,
		Title:         e.Title,
		Developer:     e.Developer,
		Installed:     e.IsInstalled,
		InstallPath:   e.Install.InstallPath,
		Platform:      e.Install.Platform,
		Version:       e.Install.Version,
		Executable:    e.Install.Executable,
		InstallSize:   e.Install.InstallSize,
		IsDLC:         e.Install.IsDLC,
		Runner:        runnerName(store),
		Source:        library.SourceHeroicImport,
		CloudSaves:    e.CloudSave,
		CanRunOffline: e.CanRunOffline,
	}
	g.Artwork = library.Artwork{
		Cover:  e.ArtCover,
		Square: e.ArtSquare,
		Logo:   e.ArtLogo,
		Icon:   e.ArtIcon,
	}
	if e.Namespace != "" || e.StoreURL != "" {
		g.Metadata = map[string]string{}
		if e.Namespace != "" {
			g.Metadata["namespace"] = e.Namespace
		}
		if e.StoreURL != "" {
			g.Metadata["store_url"] = e.StoreURL
		}
	}
	return g, true
}

func (e HeroicGameInfo) IsDLCCheck() bool { return e.Install.IsDLC }

func runnerName(store string) string {
	switch store {
	case library.StoreEpic:
		return library.RunnerLegendary
	case library.StoreGOG:
		return library.RunnerGogdl
	case library.StoreAmazon:
		return library.RunnerNile
	}
	return library.RunnerNone
}

func readHeroicGameInfos(path string) []HeroicGameInfo {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []HeroicGameInfo
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	return entries
}

func (i *Importer) mergeLegendaryInstalled(games map[string]library.Game) {
	b, err := os.ReadFile(filepath.Join(i.root, "legendaryConfig", "legendary", "installed.json"))
	if err != nil {
		return
	}
	var installed map[string]struct {
		InstallPath string `json:"install_path"`
		Executable  string `json:"executable"`
		Version     string `json:"version"`
		Platform    string `json:"platform"`
		InstallSize string `json:"install_size"`
	}
	if json.Unmarshal(b, &installed) != nil {
		return
	}
	for app, meta := range installed {
		key := library.StoreEpic + "/" + app
		if g, ok := games[key]; ok {
			g.Installed = true
			if g.InstallPath == "" {
				g.InstallPath = meta.InstallPath
			}
			games[key] = g
			continue
		}
		games[key] = library.Game{
			Store: library.StoreEpic, ID: app, Title: app,
			Installed: true, InstallPath: meta.InstallPath,
			Platform: meta.Platform, Version: meta.Version,
			Executable: meta.Executable, InstallSize: meta.InstallSize,
			Runner: library.RunnerLegendary, Source: library.SourceHeroicImport,
		}
	}
}

func (i *Importer) mergeGOGInstalled(games map[string]library.Game) {
	b, err := os.ReadFile(filepath.Join(i.root, "gog_store", "installed.json"))
	if err != nil {
		return
	}
	var wrapper struct {
		Installed []struct {
			AppName     string `json:"appName"`
			InstallPath string `json:"install_path"`
			Platform    string `json:"platform"`
			Version     string `json:"version"`
			Executable  string `json:"executable"`
		} `json:"installed"`
	}
	if json.Unmarshal(b, &wrapper) != nil {
		return
	}
	for _, meta := range wrapper.Installed {
		if meta.AppName == "" {
			continue
		}
		key := library.StoreGOG + "/" + meta.AppName
		if g, ok := games[key]; ok {
			g.Installed = true
			if g.InstallPath == "" {
				g.InstallPath = meta.InstallPath
			}
			games[key] = g
			continue
		}
		games[key] = library.Game{
			Store: library.StoreGOG, ID: meta.AppName, Title: meta.AppName,
			Installed: true, InstallPath: meta.InstallPath,
			Platform: meta.Platform, Version: meta.Version,
			Executable: meta.Executable,
			Runner:     library.RunnerGogdl, Source: library.SourceHeroicImport,
		}
	}
}

func (i *Importer) mergeNileInstalled(games map[string]library.Game) {
	b, err := os.ReadFile(filepath.Join(i.root, "nile_config", "nile", "installed.json"))
	if err != nil {
		return
	}
	var installed []struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		Path    string `json:"path"`
	}
	if json.Unmarshal(b, &installed) != nil {
		return
	}
	for _, meta := range installed {
		if meta.ID == "" {
			continue
		}
		key := library.StoreAmazon + "/" + meta.ID
		if g, ok := games[key]; ok {
			g.Installed = true
			if g.InstallPath == "" {
				g.InstallPath = meta.Path
			}
			games[key] = g
			continue
		}
		games[key] = library.Game{
			Store: library.StoreAmazon, ID: meta.ID, Title: meta.ID,
			Installed: true, InstallPath: meta.Path,
			Platform: "Windows", Version: meta.Version,
			Runner: library.RunnerNile, Source: library.SourceHeroicImport,
		}
	}
}

func (i *Importer) DefaultSettings(g library.Game) (config.GameSettings, bool) {
	switch g.Store {
	case library.StoreEpic, library.StoreGOG, library.StoreAmazon:
	default:
		return config.GameSettings{}, false
	}
	path := filepath.Join(i.root, "GamesConfig", g.ID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return config.GameSettings{}, false
	}
	var wrapper map[string]json.RawMessage
	if json.Unmarshal(b, &wrapper) != nil {
		return config.GameSettings{}, false
	}
	raw, ok := wrapper[g.ID]
	if !ok {
		return config.GameSettings{}, false
	}
	var hs heroicGameSettings
	if json.Unmarshal(raw, &hs) != nil {
		return config.GameSettings{}, false
	}
	return hs.toGameSettings(), true
}

type heroicGameSettings struct {
	WinePrefix  string `json:"winePrefix"`
	WineVersion *struct {
		Bin  string `json:"bin"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"wineVersion"`
	TargetExe         string `json:"targetExe"`
	LauncherArgs      string `json:"launcherArgs"`
	EnviromentOptions []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"enviromentOptions"`
	WrapperOptions []struct {
		Exe  string `json:"exe"`
		Args string `json:"args"`
	} `json:"wrapperOptions"`
	UseGameMode            bool   `json:"useGameMode"`
	ShowMangohud           bool   `json:"showMangohud"`
	EnableEsync            bool   `json:"enableEsync"`
	EnableFsync            bool   `json:"enableFsync"`
	EnableFSR              bool   `json:"enableFSR"`
	MaxSharpness           *int   `json:"maxSharpness"`
	AutoInstallDxvk        bool   `json:"autoInstallDxvk"`
	AutoInstallVkd3d       bool   `json:"autoInstallVkd3d"`
	AutoInstallDxvkNvapi   bool   `json:"autoInstallDxvkNvapi"`
	EacRuntime             bool   `json:"eacRuntime"`
	BattlEyeRuntime        bool   `json:"battlEyeRuntime"`
	NvidiaPrime            bool   `json:"nvidiaPrime"`
	OfflineMode            bool   `json:"offlineMode"`
	DisableUMU             bool   `json:"disableUMU"`
	Language               string `json:"language"`
	BeforeLaunchScriptPath string `json:"beforeLaunchScriptPath"`
	AfterLaunchScriptPath  string `json:"afterLaunchScriptPath"`
}

func (h heroicGameSettings) toGameSettings() config.GameSettings {
	s := config.GameSettings{
		WinePrefix:           h.WinePrefix,
		TargetExe:            h.TargetExe,
		LauncherArgs:         h.LauncherArgs,
		UseGameMode:          h.UseGameMode,
		ShowMangoHud:         h.ShowMangohud,
		EnableEsync:          h.EnableEsync,
		EnableFsync:          h.EnableFsync,
		EnableFSR:            h.EnableFSR,
		AutoInstallDxvk:      h.AutoInstallDxvk,
		AutoInstallVkd3d:     h.AutoInstallVkd3d,
		AutoInstallDxvkNvapi: h.AutoInstallDxvkNvapi,
		EacRuntime:           h.EacRuntime,
		BattlEyeRuntime:      h.BattlEyeRuntime,
		NvidiaPrime:          h.NvidiaPrime,
		OfflineMode:          h.OfflineMode,
		DisableUMU:           h.DisableUMU,
		Language:             h.Language,
		BeforeScript:         h.BeforeLaunchScriptPath,
		AfterScript:          h.AfterLaunchScriptPath,
	}
	if h.MaxSharpness != nil {
		s.MaxSharpness = *h.MaxSharpness
	}
	if h.WineVersion != nil {
		s.WineVersion = config.WineVersion{
			Bin: h.WineVersion.Bin, Name: h.WineVersion.Name, Type: h.WineVersion.Type,
		}
	}
	for _, e := range h.EnviromentOptions {
		s.Environment = append(s.Environment, config.EnvVar{Key: e.Key, Value: e.Value})
	}
	for _, w := range h.WrapperOptions {
		s.Wrappers = append(s.Wrappers, config.Wrapper{Exe: w.Exe, Args: w.Args})
	}
	return s
}

func (i *Importer) Root() string { return i.root }
