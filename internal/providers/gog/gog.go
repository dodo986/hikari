package gog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hikari/internal/atomicfile"
	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
	"hikari/internal/providers/common"
)

const loginURL = "https://auth.gog.com/auth?client_id=46899977096215655&redirect_uri=https%3A%2F%2Fembed.gog.com%2Fon_login_success%3Forigin%3Dclient&response_type=code&layout=galaxy"

type Provider struct {
	eng   *engine.Manager
	run   *engine.Runner
	paths config.Paths

	galaxy *GalaxyClient

	RunInPrefix func(ctx context.Context, settings config.GameSettings, argv []string, installDir string) error
}

func NewWithGalaxy(eng *engine.Manager, run *engine.Runner, paths config.Paths, g *GalaxyClient) *Provider {
	p := New(eng, run, paths)
	p.galaxy = g
	return p
}

func New(eng *engine.Manager, run *engine.Runner, paths config.Paths) *Provider {
	return &Provider{eng: eng, run: run, paths: paths}
}

func (p *Provider) Store() string { return library.StoreGOG }

func (p *Provider) dataDir() string { return filepath.Join(p.paths.EngineGOG, "heroic_gogdl") }

func (p *Provider) authConfig() string { return filepath.Join(p.paths.EngineGOG, "auth.json") }

func (p *Provider) cacheFile() string { return filepath.Join(p.paths.DataDir, "libraries", "gog.json") }

func (p *Provider) supportDir(id string) string {
	return filepath.Join(p.dataDir(), "gog-support", id)
}

type entry struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	InstallPath     string `json:"install_path,omitempty"`
	Platform        string `json:"platform,omitempty"`
	Version         string `json:"version,omitempty"`
	Language        string `json:"language,omitempty"`
	BuildID         string `json:"build_id,omitempty"`
	Installed       bool   `json:"installed"`
	Developer       string `json:"developer,omitempty"`
	Cover           string `json:"cover,omitempty"`
	Square          string `json:"square,omitempty"`
	Icon            string `json:"icon,omitempty"`
	LibraryPlatform string `json:"library_platform,omitempty"`
}

var usersBase = "https://users.gog.com"

func (p *Provider) usernameCache() string {
	return filepath.Join(p.paths.EngineGOG, "user.json")
}

func (p *Provider) Username(ctx context.Context) (string, error) {
	if b, err := os.ReadFile(p.usernameCache()); err == nil {
		var cached struct {
			Username string `json:"username"`
		}
		if json.Unmarshal(b, &cached) == nil && cached.Username != "" {
			return cached.Username, nil
		}
	}
	creds, _ := p.credentials()
	if creds.AccessToken == "" || creds.UserID == "" {
		return "", fmt.Errorf("not logged in to GOG")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usersBase+"/users/"+creds.UserID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("users.gog.com: status %d", resp.StatusCode)
	}
	var user struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	if user.Username != "" {
		b, _ := json.MarshalIndent(map[string]string{"username": user.Username}, "", "  ")
		_ = os.MkdirAll(filepath.Dir(p.usernameCache()), 0o755)
		_ = os.WriteFile(p.usernameCache(), append(b, '\n'), 0o600)
	}
	return user.Username, nil
}

func (p *Provider) credentials() (Credentials, error) {
	var creds Credentials
	b, err := os.ReadFile(p.authConfig())
	if err != nil {
		return creds, nil
	}
	_ = json.Unmarshal(b, &creds)
	return creds, nil
}

func (p *Provider) readCache() []entry {
	b, err := os.ReadFile(p.cacheFile())
	if err != nil {
		return nil
	}
	var entries []entry
	if json.Unmarshal(b, &entries) != nil {
		return nil
	}
	return entries
}

func (p *Provider) writeCache(entries []entry) error {
	if err := os.MkdirAll(filepath.Dir(p.cacheFile()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(p.cacheFile(), append(b, '\n'))
}

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	entries := p.readCache()
	games := make([]library.Game, 0, len(entries))
	for _, e := range entries {
		g := library.Game{
			Store:       library.StoreGOG,
			ID:          e.ID,
			Title:       e.Title,
			Installed:   e.Installed,
			InstallPath: e.InstallPath,
			Platform:    e.Platform,
			Version:     e.Version,
			Developer:   e.Developer,
			Runner:      library.RunnerGogdl,
			Source:      library.SourceHikari,
			Artwork:     library.Artwork{Cover: e.Cover, Square: e.Square, Icon: e.Icon},
		}
		if g.Title == "" {
			g.Title = e.ID
		}
		if !g.Installed && e.LibraryPlatform != "" {
			g.Platform = e.LibraryPlatform
		}
		games = append(games, g)
	}
	return games, nil
}

func (p *Provider) runGogdl(ctx context.Context, args []string, onLine func(string), sanitize func(string) string) engine.Result {
	bin, err := p.eng.Require(engine.Gogdl)
	if err != nil {
		return engine.Result{Err: err}
	}
	full := append(engine.GogdlAuthHeader(p.authConfig()), args...)
	opts := engine.RunOptions{Env: engine.EnvFor(engine.Gogdl, p.paths)}
	if onLine != nil {
		opts.OnLine = onLine
	}
	if sanitize != nil {
		opts.Sanitize = sanitize
	}
	return p.run.Run(ctx, bin, full, opts)
}

type gogdlInfo struct {
	Game struct {
		AppName string `json:"app_name"`
		Title   string `json:"title"`
		Version string `json:"version"`
		BuildID string `json:"buildId"`
	} `json:"game"`
	Download struct {
		Languages   []string `json:"languages"`
		FolderName  string   `json:"folder_name"`
		BuildID     string   `json:"buildId"`
		VersionName string   `json:"versionName"`
		Size        map[string]struct {
			DownloadSize float64 `json:"download_size"`
			DiskSize     float64 `json:"disk_size"`
		} `json:"size"`
		DLCs []struct {
			Title string `json:"title"`
			ID    string `json:"id"`
			Size  map[string]struct {
				DownloadSize float64 `json:"download_size"`
			} `json:"size"`
		} `json:"dlcs"`
	} `json:"download"`
}

type gogInfoFile struct {
	Name      string `json:"name"`
	PlayTasks []struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		IsHidden bool   `json:"isHidden"`
		Category string `json:"category"`
	} `json:"playTasks"`
}

func (p *Provider) LaunchOptions(ctx context.Context, g library.Game) ([]library.LaunchOption, error) {
	if g.InstallPath == "" {
		for _, e := range p.readCache() {
			if e.ID == g.ID {
				g.InstallPath = e.InstallPath
				break
			}
		}
	}
	if g.InstallPath == "" {
		return nil, fmt.Errorf("the game's install directory is unknown")
	}
	b, err := os.ReadFile(filepath.Join(g.InstallPath, "goggame-"+g.ID+".info"))
	if err != nil {
		return nil, fmt.Errorf("no goggame info file for %s", g.Title)
	}
	var info gogInfoFile
	if err := json.Unmarshal(b, &info); err != nil {
		return nil, fmt.Errorf("goggame info file is not valid JSON: %w", err)
	}
	var out []library.LaunchOption
	for i, task := range info.PlayTasks {
		if task.Type != "FileTask" || task.IsHidden || task.Category == "document" {
			continue
		}
		name := task.Name
		if name == "" {
			name = info.Name
		}
		out = append(out, library.LaunchOption{
			Name: name, Type: "basic",
			Params: []string{"--prefer-task", fmt.Sprint(i)},
		})
	}
	if len(out) < 2 {
		return nil, nil
	}
	return out, nil
}

func (p *Provider) Repair(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	var target *entry
	for _, e := range p.readCache() {
		if e.ID == id && e.Installed {
			c := e
			target = &c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("GOG game %q is not installed through Hikari", id)
	}
	if progress != nil {
		progress(library.InstallProgress{State: "repairing"})
	}
	parser := &engine.Parser{}
	platform := orDefault(target.Platform, "windows")
	res := p.runGogdl(ctx, engine.GogdlRepairArgs(id, platform, target.InstallPath, p.supportDir(id), target.Language, target.BuildID, opts.MaxWorkers), func(line string) {
		if progress == nil {
			return
		}
		if frame, emit := parser.Parse(line); emit {
			progress(common.ProgressFromFrame("repairing", frame))
		}
	}, nil)
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("gogdl repair failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) InstallInfo(ctx context.Context, id, platform string) (library.InstallInfo, error) {
	if platform == "" {
		platform = "windows"
	} else {
		platform = strings.ToLower(platform)
	}
	res := p.runGogdl(ctx, []string{"info", id, "--os", platform}, nil, nil)
	if res.Err != nil {
		return library.InstallInfo{}, fmt.Errorf("gogdl info failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 3))
	}
	var info gogdlInfo
	if err := json.Unmarshal([]byte(res.Stdout), &info); err != nil {
		return library.InstallInfo{}, fmt.Errorf("gogdl info output not JSON: %w", err)
	}
	out := library.InstallInfo{
		Title:   info.Game.Title,
		Version: orDefault(info.Download.VersionName, info.Game.Version),
	}
	if star, ok := info.Download.Size["*"]; ok {
		out.DownloadSize = int64(star.DownloadSize)
		out.DiskSize = int64(star.DiskSize)
	}
	for _, d := range info.Download.DLCs {
		dlc := library.DLC{ID: d.ID, Title: d.Title}
		if star, ok := d.Size["*"]; ok {
			dlc.DownloadSize = int64(star.DownloadSize)
		}
		out.DLCs = append(out.DLCs, dlc)
	}
	return out, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (p *Provider) Install(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	if opts.Path == "" {
		return errors.New("install path is required for GOG games")
	}
	platform := opts.Platform
	switch platform {
	case "":
		platform = "windows"
	case "Mac":
		platform = "osx"
	case "Windows", "windows", "Linux", "linux", "osx":
		if platform[0] >= 'A' && platform[0] <= 'Z' {
			platform = strings.ToLower(platform)
		}
	default:
		return fmt.Errorf("unsupported platform %q", opts.Platform)
	}
	if progress != nil {
		progress(library.InstallProgress{State: "downloading"})
	}

	if res := p.runGogdl(ctx, engine.GogdlAuthRefresh(), nil, common.RedactLine("refresh_token")); res.Err != nil {
		return fmt.Errorf("gogdl auth refresh failed; are you logged in? (%s)", common.FirstLines(res.Stderr, 3))
	}

	var info gogdlInfo
	if res := p.runGogdl(ctx, []string{"info", id, "--os", platform}, nil, nil); res.Err == nil {
		_ = json.Unmarshal([]byte(res.Stdout), &info)
	}

	parser := &engine.Parser{}
	args := engine.GogdlDownloadArgs(id, platform, opts.Path, p.supportDir(id), opts.Language, opts.DLCs, opts.MaxWorkers)
	res := p.runGogdl(ctx, args, func(line string) {
		if progress == nil {
			return
		}
		if frame, emit := parser.Parse(line); emit {
			progress(common.ProgressFromFrame("downloading", frame))
		}
	}, nil)
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("gogdl download failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}

	title := info.Game.Title
	if title == "" {
		title = id
	}
	p.upsertCache(entry{
		ID: id, Title: title, InstallPath: opts.Path, Platform: platform,
		Version: info.Download.VersionName, BuildID: info.Download.BuildID,
		Language: opts.Language, Installed: true,
	})
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) upsertCache(e entry) {
	entries := p.readCache()
	for i := range entries {
		if entries[i].ID == e.ID {
			entries[i] = e
			_ = p.writeCache(entries)
			return
		}
	}
	_ = p.writeCache(append(entries, e))
}

func (p *Provider) Update(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	var target *entry
	for _, e := range p.readCache() {
		if e.ID == id && e.Installed {
			c := e
			target = &c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("GOG game %q is not installed through Hikari", id)
	}
	platform := orDefault(target.Platform, "windows")
	if progress != nil {
		progress(library.InstallProgress{State: "updating"})
	}
	parser := &engine.Parser{}
	res := p.runGogdl(ctx, engine.GogdlUpdateArgs(id, platform, target.InstallPath, p.supportDir(id), target.Language, target.BuildID, opts.MaxWorkers), func(line string) {
		if progress == nil {
			return
		}
		if frame, emit := parser.Parse(line); emit {
			progress(common.ProgressFromFrame("updating", frame))
		}
	}, nil)
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("gogdl update failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Uninstall(ctx context.Context, id string, keepFiles bool) error {
	var target *entry
	for _, e := range p.readCache() {
		if e.ID == id {
			c := e
			target = &c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("GOG game %q is not known to Hikari; only Hikari-installed games can be uninstalled", id)
	}
	if target.InstallPath != "" && !keepFiles {
		uninstaller := filepath.Join(target.InstallPath, "unins000.exe")
		if _, err := os.Stat(uninstaller); err == nil && p.RunInPrefix != nil {
			_ = p.RunInPrefix(ctx, config.GameSettings{}, []string{
				uninstaller, "/VERYSILENT", "/ProductId=" + id, "/galaxyclient", "/KEEPSAVES",
			}, target.InstallPath)
		}
		if err := os.RemoveAll(target.InstallPath); err != nil {
			return fmt.Errorf("cannot remove install dir %s: %w", target.InstallPath, err)
		}
	}
	_ = os.RemoveAll(filepath.Join(p.dataDir(), "manifests", id))
	_ = os.RemoveAll(p.supportDir(id))
	target.Installed = false
	target.InstallPath = ""
	p.upsertCache(*target)
	return nil
}

func (p *Provider) AuthStatus(ctx context.Context) (library.AuthStatus, error) {
	st := library.AuthStatus{}
	b, err := os.ReadFile(p.authConfig())
	if err != nil {
		return st, nil
	}
	var creds struct {
		UserID string `json:"user_id"`
	}
	_ = json.Unmarshal(b, &creds)
	st.LoggedIn = len(b) > 2
	st.UserID = creds.UserID
	return st, nil
}

func (p *Provider) BeginLogin(ctx context.Context) (library.LoginInfo, error) {
	return library.LoginInfo{
		URL:  loginURL,
		Hint: "open the URL, log in to GOG, then copy the code parameter from the final embed.gog.com/on_login_success URL",
	}, nil
}

func (p *Provider) CompleteLogin(ctx context.Context, code string) (library.AuthStatus, error) {
	res := p.runGogdl(ctx, engine.GogdlAuthCode(code), nil, common.RedactLine("access_token"))
	if res.Err != nil {
		return library.AuthStatus{}, fmt.Errorf("GOG login failed: %s", common.FirstLines(res.Stderr, 5))
	}
	var reply struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &reply); err == nil && reply.Error != "" {
		return library.AuthStatus{}, fmt.Errorf("GOG login failed: %s", reply.Error)
	}
	return p.AuthStatus(ctx)
}

func (p *Provider) Logout(ctx context.Context) error {
	if err := os.Remove(p.authConfig()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *Provider) Refresh(ctx context.Context) (int, error) {
	creds, err := p.credentials()
	if err != nil || creds.AccessToken == "" {
		games, err := p.List(ctx)
		if err != nil {
			return 0, err
		}
		return len(games), nil
	}
	galaxy := p.galaxy
	if galaxy == nil {
		galaxy = NewGalaxyClient()
	}
	entries, err := galaxy.Library(ctx, creds)
	if err != nil {
		return 0, fmt.Errorf("GOG library fetch failed: %w", err)
	}

	installed := map[string]entry{}
	for _, e := range p.readCache() {
		if e.Installed {
			installed[e.ID] = e
		}
	}
	var merged []entry
	seen := map[string]bool{}
	for _, rel := range entries {
		if rel.PlatformID != "gog" {
			continue
		}
		meta, err := galaxy.GameMeta(ctx, creds, rel.ExternalID, rel.Certificate)
		if err != nil || meta == nil {
			continue
		}
		g, ok := meta.toGame()
		if !ok {
			continue
		}
		e := entry{
			ID: g.ID, Title: g.Title,
			Developer: g.Developer, Platform: g.Platform,
			Cover: g.Artwork.Cover, Square: g.Artwork.Square, Icon: g.Artwork.Icon,
			LibraryPlatform: g.Platform,
		}
		if inst, have := installed[g.ID]; have {
			inst.Title = e.Title
			inst.Developer = e.Developer
			inst.Cover, inst.Square, inst.Icon = e.Cover, e.Square, e.Icon
			inst.LibraryPlatform = e.LibraryPlatform
			e = inst
		}
		merged = append(merged, e)
		seen[g.ID] = true
	}
	for _, e := range p.readCache() {
		if !seen[e.ID] && e.Installed {
			merged = append(merged, e)
		}
	}
	if len(merged) == 0 {
		return 0, fmt.Errorf("GOG library came back empty; keeping the local cache")
	}
	if err := p.writeCache(merged); err != nil {
		return 0, err
	}
	return len(merged), nil
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "gog-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
