package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
	"hikari/internal/providers/common"
)

const loginURL = "https://legendary.gl/epiclogin"

type Provider struct {
	eng   *engine.Manager
	run   *engine.Runner
	paths config.Paths
}

func New(eng *engine.Manager, run *engine.Runner, paths config.Paths) *Provider {
	return &Provider{eng: eng, run: run, paths: paths}
}

func (p *Provider) Store() string { return library.StoreEpic }

type metadataFile struct {
	Metadata struct {
		Title       string `json:"title"`
		Developer   string `json:"developer"`
		Namespace   string `json:"namespace"`
		Description string `json:"description"`
		KeyImages   []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"keyImages"`
		Categories []struct {
			Path string `json:"path"`
		} `json:"categories"`
		DLCItemList []struct {
			ReleaseInfo []struct {
				AppID string `json:"appId"`
			} `json:"releaseInfo"`
		} `json:"dlcItemList"`
		ReleaseInfo []struct {
			Platform []string `json:"platform"`
		} `json:"releaseInfo"`
		MainGameItem *struct {
			ReleaseInfo []struct {
				Platform []string `json:"platform"`
			} `json:"releaseInfo"`
		} `json:"mainGameItem"`
		CustomAttributes map[string]struct {
			Value string `json:"value"`
		} `json:"customAttributes"`
	} `json:"metadata"`
}

type installedEntry struct {
	InstallPath string   `json:"install_path"`
	Executable  string   `json:"executable"`
	Version     string   `json:"version"`
	Platform    string   `json:"platform"`
	InstallSize byteSize `json:"install_size"`
	SavePath    string   `json:"save_path"`
}

type byteSize float64

func (b *byteSize) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), "\"")
	if s == "" || s == "null" {
		*b = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*b = 0
		return nil
	}
	*b = byteSize(v)
	return nil
}

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	installed := p.readInstalled()
	metas := p.readMetadata()
	assets := p.readAssetVersions()
	games := make([]library.Game, 0, len(metas))
	for app, meta := range metas {
		var inst *installedEntry
		if e, ok := installed[app]; ok {
			inst = &e
		}
		g, ok := p.toGame(app, meta, inst)
		if !ok {
			continue
		}
		if g.Installed && g.Version != "" {
			if latest, ok := assets[g.Platform+"/"+app]; ok && latest != "" && latest != g.Version {
				g.UpdateAvailable = true
			}
		}
		games = append(games, g)
	}
	for app, inst := range installed {
		if _, have := metas[app]; have {
			continue
		}
		games = append(games, library.Game{
			Store: library.StoreEpic, ID: app, Title: app,
			Installed: true, InstallPath: inst.InstallPath,
			Platform: inst.Platform, Version: inst.Version,
			Executable: inst.Executable, InstallSize: common.HumanSize(float64(inst.InstallSize)),
			Runner: library.RunnerLegendary, Source: library.SourceHikari,
		})
	}
	return games, nil
}

func (p *Provider) readAssetVersions() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineEpic, "assets.json"))
	if err != nil {
		return out
	}
	var assets map[string][]struct {
		AppName      string `json:"app_name"`
		BuildVersion string `json:"build_version"`
	}
	if json.Unmarshal(b, &assets) != nil {
		return out
	}
	for platform, list := range assets {
		for _, a := range list {
			out[platform+"/"+a.AppName] = a.BuildVersion
		}
	}
	return out
}

func (p *Provider) LaunchOptions(ctx context.Context, g library.Game) ([]library.LaunchOption, error) {
	platform := g.Platform
	if platform == "" {
		platform = "Windows"
	}
	info, err := p.getInstallInfo(ctx, g.ID, platform)
	if err != nil {
		return nil, err
	}
	var out []library.LaunchOption
	for _, lo := range info.Game.LaunchOptions {
		name := lo.Name
		if name == "" {
			name = "Default"
		}
		out = append(out, library.LaunchOption{
			Name: name, Type: "basic",
			Params: strings.Fields(lo.Parameters),
		})
	}
	installed := p.readInstalled()
	for _, dlc := range info.Game.OwnedDLC {
		inst, ok := installed[dlc.AppName]
		if !ok || inst.Executable == "" {
			continue
		}
		out = append(out, library.LaunchOption{
			Name: dlc.Title, Type: "dlc", DLCApp: dlc.AppName,
		})
	}
	return out, nil
}

func (p *Provider) Repair(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(library.InstallProgress{State: "repairing"})
	}
	parser := &engine.Parser{}
	res := p.run.Run(ctx, bin, engine.LegendaryRepairArgs(id, opts.MaxWorkers), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
		OnLine: func(line string) {
			if progress == nil {
				return
			}
			if frame, emit := parser.Parse(line); emit {
				progress(common.ProgressFromFrame("repairing", frame))
			}
		},
	})
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("legendary repair failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) SyncSaves(ctx context.Context, id string, opts library.SaveOptions) (string, error) {
	if opts.SavePath == "" {
		return "", fmt.Errorf("set the game's save path in its settings first")
	}
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	res := p.run.Run(ctx, bin, engine.LegendarySyncSavesArgs(id, opts.SavePath, opts.Direction), engine.RunOptions{
		Env:    engine.EnvFor(engine.Legendary, p.paths),
		OnLine: func(line string) { out.WriteString(line + "\n") },
	})
	if res.Err != nil {
		return out.String(), fmt.Errorf("legendary sync-saves failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return out.String(), nil
}

func (p *Provider) Update(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(library.InstallProgress{State: "updating"})
	}
	parser := &engine.Parser{}
	res := p.run.Run(ctx, bin, engine.LegendaryUpdateArgs(id, opts.MaxWorkers), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
		OnLine: func(line string) {
			if progress == nil {
				return
			}
			if frame, emit := parser.Parse(line); emit {
				progress(common.ProgressFromFrame("updating", frame))
			}
		},
	})
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("legendary update failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Refresh(ctx context.Context) (int, error) {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return 0, err
	}
	res := p.run.Run(ctx, bin, engine.LegendaryList(), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
	})
	if res.Err != nil {
		return 0, fmt.Errorf("legendary list failed: %w", res.Err)
	}
	games, err := p.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(games), nil
}

func (p *Provider) readInstalled() map[string]installedEntry {
	out := map[string]installedEntry{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineEpic, "installed.json"))
	if err != nil {
		return out
	}
	var m map[string]installedEntry
	if json.Unmarshal(b, &m) != nil {
		return out
	}
	return m
}

func (p *Provider) readMetadata() map[string]metadataFile {
	out := map[string]metadataFile{}
	entries, err := os.ReadDir(filepath.Join(p.paths.EngineEpic, "metadata"))
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		app := strings.TrimSuffix(e.Name(), ".json")
		b, err := os.ReadFile(filepath.Join(p.paths.EngineEpic, "metadata", e.Name()))
		if err != nil {
			continue
		}
		var mf metadataFile
		if json.Unmarshal(b, &mf) != nil {
			continue
		}
		out[app] = mf
	}
	return out
}

func (p *Provider) toGame(app string, mf metadataFile, inst *installedEntry) (library.Game, bool) {
	m := mf.Metadata

	if m.Namespace == "ue" {
		return library.Game{}, false
	}
	ueCategories := map[string]bool{"assets": true, "asset-format": true, "plugins": true, "projects": true}
	for _, c := range m.Categories {
		if ueCategories[c.Path] {
			return library.Game{}, false
		}
		if c.Path == "mods" {
			return library.Game{}, false
		}
	}
	if len(m.ReleaseInfo) > 0 && len(m.ReleaseInfo[0].Platform) > 0 {
		mobileOnly := true
		for _, plat := range m.ReleaseInfo[0].Platform {
			if plat != "Android" && plat != "iOS" {
				mobileOnly = false
				break
			}
		}
		if mobileOnly {
			return library.Game{}, false
		}
	}

	g := library.Game{
		Store:     library.StoreEpic,
		ID:        app,
		Title:     m.Title,
		Developer: m.Developer,
		Runner:    library.RunnerLegendary,
		Source:    library.SourceHikari,
		IsDLC:     m.MainGameItem != nil,
	}
	if g.IsDLC {
		return library.Game{}, false
	}
	if m.Namespace != "" {
		g.Metadata = map[string]string{"namespace": m.Namespace}
	}

	for _, img := range m.KeyImages {
		switch img.Type {
		case "DieselGameBox", "OfferImageWide":
			if g.Artwork.Cover == "" {
				g.Artwork.Cover = img.URL
			}
		case "DieselGameBoxTall", "OfferImageTall":
			if g.Artwork.Square == "" {
				g.Artwork.Square = img.URL
			}
		case "DieselGameBoxLogo":
			g.Artwork.Logo = img.URL
		}
	}

	if ca := m.CustomAttributes; ca != nil {
		if v, ok := ca["CanRunOffline"]; ok {
			g.CanRunOffline = v.Value == "true"
		}
		if _, ok := ca["CloudSaveFolder"]; ok {
			g.CloudSaves = true
		}
	}

	if inst != nil {
		g.Installed = true
		g.InstallPath = inst.InstallPath
		g.Platform = inst.Platform
		g.Version = inst.Version
		g.Executable = inst.Executable
		g.InstallSize = common.HumanSize(float64(inst.InstallSize))
	}
	return g, true
}

type installInfo struct {
	Game struct {
		Title         string `json:"title"`
		Version       string `json:"version"`
		LaunchOptions []struct {
			Name       string `json:"name"`
			Parameters string `json:"parameters"`
		} `json:"launch_options"`
		OwnedDLC []struct {
			AppName string `json:"app_name"`
			Title   string `json:"title"`
		} `json:"owned_dlc"`
	} `json:"game"`
	Manifest struct {
		DownloadSize float64 `json:"download_size"`
		DiskSize     float64 `json:"disk_size"`
	} `json:"manifest"`
}

func (p *Provider) InstallInfo(ctx context.Context, id, platform string) (library.InstallInfo, error) {
	if platform == "" {
		platform = "Windows"
	}
	info, err := p.getInstallInfo(ctx, id, platform)
	if err != nil {
		return library.InstallInfo{}, err
	}
	out := library.InstallInfo{
		Title:        info.Game.Title,
		Version:      info.Game.Version,
		DownloadSize: int64(info.Manifest.DownloadSize),
		DiskSize:     int64(info.Manifest.DiskSize),
	}
	for _, d := range info.Game.OwnedDLC {
		out.DLCs = append(out.DLCs, library.DLC{ID: d.AppName, Title: d.Title})
	}
	return out, nil
}

func (p *Provider) getInstallInfo(ctx context.Context, app, platform string) (installInfo, error) {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return installInfo{}, err
	}
	res := p.run.Run(ctx, bin, engine.LegendaryInfo(app, platform), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
	})
	if res.Err != nil {
		return installInfo{}, fmt.Errorf("legendary info failed: %w", res.Err)
	}
	var info installInfo
	if err := json.Unmarshal([]byte(res.Stdout), &info); err != nil {
		return installInfo{}, fmt.Errorf("legendary info output not JSON: %w", err)
	}
	return info, nil
}

func (p *Provider) Install(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return err
	}
	platform := opts.Platform
	if platform == "" {
		platform = "Windows"
	}
	if progress != nil {
		progress(library.InstallProgress{State: "downloading"})
	}
	var totalMiB *float64
	if info, err := p.getInstallInfo(ctx, id, platform); err == nil && info.Manifest.DownloadSize > 0 {
		mib := info.Manifest.DownloadSize / 1024 / 1024
		totalMiB = &mib
	}

	parser := &engine.Parser{}
	if totalMiB != nil {
		parser.SetTotalMiB(*totalMiB)
	}
	args := engine.LegendaryInstallArgs(id, opts.Path, platform, opts.MaxWorkers, nil)
	res := p.run.Run(ctx, bin, args, engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
		OnLine: func(line string) {
			if progress == nil {
				return
			}
			if frame, emit := parser.Parse(line); emit {
				progress(common.ProgressFromFrame("downloading", frame))
			}
		},
	})
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("legendary install failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Uninstall(ctx context.Context, id string, keepFiles bool) error {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return err
	}
	res := p.run.Run(ctx, bin, engine.LegendaryUninstallArgs(id, keepFiles), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
	})
	if res.Err != nil {
		return fmt.Errorf("legendary uninstall failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return nil
}

func (p *Provider) AuthStatus(ctx context.Context) (library.AuthStatus, error) {
	st := library.AuthStatus{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineEpic, "user.json"))
	if err != nil {
		return st, nil
	}
	var user struct {
		DisplayName string `json:"displayName"`
		AccountID   string `json:"account_id"`
	}
	if json.Unmarshal(b, &user) != nil {
		return st, nil
	}
	st.LoggedIn = true
	st.User = user.DisplayName
	st.UserID = user.AccountID
	return st, nil
}

func (p *Provider) BeginLogin(ctx context.Context) (library.LoginInfo, error) {
	return library.LoginInfo{
		URL:  loginURL,
		Hint: "open the URL, log in to Epic, copy the code it shows and pass it back as the login code",
	}, nil
}

func (p *Provider) CompleteLogin(ctx context.Context, code string) (library.AuthStatus, error) {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return library.AuthStatus{}, err
	}
	res := p.run.Run(ctx, bin, engine.LegendaryAuthCode(code), engine.RunOptions{
		Env:      engine.EnvFor(engine.Legendary, p.paths),
		Sanitize: common.RedactLine("--code"),
	})
	if res.Err != nil || strings.Contains(res.Stderr, "ERROR: Logging in") {
		return library.AuthStatus{}, fmt.Errorf("epic login failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return p.AuthStatus(ctx)
}

func (p *Provider) Logout(ctx context.Context) error {
	bin, err := p.eng.Require(engine.Legendary)
	if err != nil {
		return err
	}
	res := p.run.Run(ctx, bin, engine.LegendaryAuthDelete(), engine.RunOptions{
		Env: engine.EnvFor(engine.Legendary, p.paths),
	})
	if res.Err != nil {
		return fmt.Errorf("epic logout failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return nil
}
