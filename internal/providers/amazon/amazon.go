package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
	"hikari/internal/providers/common"
)

var registerSuccess = regexp.MustCompile(`\[AUTH_MANAGER\]:.*Succesfully registered a device`)

type Provider struct {
	eng   *engine.Manager
	run   *engine.Runner
	paths config.Paths

	mu      sync.Mutex
	pending *nileLoginData
}

func New(eng *engine.Manager, run *engine.Runner, paths config.Paths) *Provider {
	return &Provider{eng: eng, run: run, paths: paths}
}

func (p *Provider) Store() string { return library.StoreAmazon }

type nileLibraryEntry struct {
	Product struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		ProductDetail struct {
			Details struct {
				ShortDescription string `json:"shortDescription"`
				Developer        string `json:"developer"`
				BackgroundUrl1   string `json:"backgroundUrl1"`
				BackgroundUrl2   string `json:"backgroundUrl2"`
				LogoUrl          string `json:"logoUrl"`
			} `json:"details"`
			IconUrl string `json:"iconUrl"`
		} `json:"productDetail"`
	} `json:"product"`
}

type nileInstalledEntry struct {
	ID      string  `json:"id"`
	Version string  `json:"version"`
	Path    string  `json:"path"`
	Size    float64 `json:"size"`
}

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	library_ := p.readLibrary()
	installed := p.readInstalled()
	games := make([]library.Game, 0, len(library_))
	for _, e := range library_ {
		if e.Product.ID == "" {
			continue
		}
		g := library.Game{
			Store:         library.StoreAmazon,
			ID:            e.Product.ID,
			Title:         e.Product.Title,
			Developer:     e.Product.ProductDetail.Details.Developer,
			Runner:        library.RunnerNile,
			Source:        library.SourceHikari,
			Platform:      "Windows",
			CanRunOffline: true,
			Artwork: library.Artwork{
				Square: e.Product.ProductDetail.Details.BackgroundUrl2,
				Logo:   e.Product.ProductDetail.Details.LogoUrl,
				Icon:   e.Product.ProductDetail.IconUrl,
			},
		}
		if g.Title == "" {
			g.Title = e.Product.ID
		}
		if inst, ok := installed[e.Product.ID]; ok {
			g.Installed = true
			g.InstallPath = inst.Path
			g.Version = inst.Version
		}
		games = append(games, g)
	}

	for id, inst := range installed {
		if _, have := library_[id]; have {
			continue
		}
		games = append(games, library.Game{
			Store: library.StoreAmazon, ID: id, Title: id,
			Installed: true, InstallPath: inst.Path, Version: inst.Version,
			Platform: "Windows", Runner: library.RunnerNile,
			Source: library.SourceHikari, CanRunOffline: true,
		})
	}
	return games, nil
}

func (p *Provider) readLibrary() map[string]nileLibraryEntry {
	out := map[string]nileLibraryEntry{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineAmazon, "library.json"))
	if err != nil {
		return out
	}
	var entries []nileLibraryEntry
	if json.Unmarshal(b, &entries) != nil {
		return out
	}
	for _, e := range entries {
		out[e.Product.ID] = e
	}
	return out
}

func (p *Provider) readInstalled() map[string]nileInstalledEntry {
	out := map[string]nileInstalledEntry{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineAmazon, "installed.json"))
	if err != nil {
		return out
	}
	var entries []nileInstalledEntry
	if json.Unmarshal(b, &entries) != nil {
		return out
	}
	for _, e := range entries {
		out[e.ID] = e
	}
	return out
}

func (p *Provider) InstallInfo(ctx context.Context, id, platform string) (library.InstallInfo, error) {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return library.InstallInfo{}, err
	}
	res := p.run.Run(ctx, bin, []string{"install", "--info", "--json", id}, engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Err != nil {
		return library.InstallInfo{}, fmt.Errorf("nile install info failed: %s", common.FirstLines(res.Stderr, 3))
	}
	var info struct {
		DownloadSize float64 `json:"download_size"`
		DiskSize     float64 `json:"disk_size"`
		Version      string  `json:"version"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &info); err != nil {
		return library.InstallInfo{}, fmt.Errorf("nile install info output not JSON: %w", err)
	}
	out := library.InstallInfo{
		DownloadSize: int64(info.DownloadSize),
		DiskSize:     int64(info.DiskSize),
		Version:      info.Version,
	}

	for _, g := range p.listLocal() {
		if g.ID == id {
			out.Title = g.Title
			if out.Version == "" {
				out.Version = g.Version
			}
			break
		}
	}
	return out, nil
}

func (p *Provider) listLocal() []library.Game {
	games, _ := p.List(context.Background())
	return games
}

func (p *Provider) Refresh(ctx context.Context) (int, error) {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return 0, err
	}
	res := p.run.Run(ctx, bin, engine.NileLibrarySync(), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Err != nil {
		return 0, fmt.Errorf("nile library sync failed: %s", common.FirstLines(res.Stderr, 5))
	}
	games, err := p.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(games), nil
}

func (p *Provider) Install(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(library.InstallProgress{State: "downloading"})
	}
	parser := &engine.Parser{}
	res := p.run.Run(ctx, bin, engine.NileInstallArgs(id, opts.Path, opts.MaxWorkers), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
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
		return fmt.Errorf("nile install failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Update(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return err
	}
	if progress != nil {
		progress(library.InstallProgress{State: "updating"})
	}
	parser := &engine.Parser{}
	res := p.run.Run(ctx, bin, engine.NileUpdateArgs(id, opts.MaxWorkers), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
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
		return fmt.Errorf("nile update failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Repair(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return err
	}
	path := ""
	for _, e := range p.readInstalled() {
		if e.ID == id {
			path = e.Path
		}
	}
	if path == "" {
		return fmt.Errorf("could not find the install path for %s", id)
	}
	if progress != nil {
		progress(library.InstallProgress{State: "verifying"})
	}
	res := p.run.Run(ctx, bin, engine.NileVerifyArgs(id, path), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Canceled {
		return common.InstallCanceled(progress)
	}
	if res.Err != nil {
		if progress != nil {
			progress(library.InstallProgress{State: "error"})
		}
		return fmt.Errorf("nile verify failed (exit %d): %s", res.ExitCode, common.FirstLines(res.Stderr, 5))
	}
	if progress != nil {
		progress(library.InstallProgress{State: "done", Percent: common.FloatPtr(100)})
	}
	return nil
}

func (p *Provider) Uninstall(ctx context.Context, id string, keepFiles bool) error {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return err
	}
	res := p.run.Run(ctx, bin, engine.NileUninstallArgs(id), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Err != nil {
		return fmt.Errorf("nile uninstall failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return nil
}

func (p *Provider) AuthStatus(ctx context.Context) (library.AuthStatus, error) {
	st := library.AuthStatus{}
	b, err := os.ReadFile(filepath.Join(p.paths.EngineAmazon, "current_user.json"))
	if err != nil {
		return st, nil
	}
	var user struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
	}
	if json.Unmarshal(b, &user) != nil || user.UserID == "" && user.Name == "" {
		return st, nil
	}
	st.LoggedIn = true
	st.User = user.Name
	st.UserID = user.UserID
	return st, nil
}

type nileLoginData struct {
	URL          string `json:"url"`
	CodeVerifier string `json:"code_verifier"`
	Serial       string `json:"serial"`
	ClientID     string `json:"client_id"`
	DeviceSecret string `json:"device_secret"`
}

func (p *Provider) BeginLogin(ctx context.Context) (library.LoginInfo, error) {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return library.LoginInfo{}, err
	}
	res := p.run.Run(ctx, bin, engine.NileAuthLogin(), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Err != nil {
		return library.LoginInfo{}, fmt.Errorf("nile auth failed: %s", common.FirstLines(res.Stderr, 5))
	}
	var data nileLoginData
	if err := json.Unmarshal([]byte(res.Stdout), &data); err != nil || data.URL == "" {
		return library.LoginInfo{}, fmt.Errorf("nile auth returned no login URL")
	}

	p.mu.Lock()
	p.pending = &data
	p.mu.Unlock()
	return library.LoginInfo{
		URL:  data.URL,
		Hint: "open the URL, log in to Amazon, and paste the code from the redirect URL's openid.oa2.authorization_code parameter",
	}, nil
}

func (p *Provider) CompleteLogin(ctx context.Context, code string) (library.AuthStatus, error) {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return library.AuthStatus{}, err
	}
	p.mu.Lock()
	data := p.pending
	p.mu.Unlock()
	if data == nil {
		return library.AuthStatus{}, fmt.Errorf("no pending amazon login; call login begin first")
	}
	res := p.run.Run(ctx, bin, engine.NileRegisterArgs(code, data.CodeVerifier, data.Serial, data.ClientID), engine.RunOptions{
		Env:      engine.EnvFor(engine.Nile, p.paths),
		Sanitize: common.RedactLine(data.CodeVerifier),
	})
	if !registerSuccess.MatchString(res.Stderr) {
		return library.AuthStatus{}, fmt.Errorf("amazon login failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return p.AuthStatus(ctx)
}

func (p *Provider) Logout(ctx context.Context) error {
	bin, err := p.eng.Require(engine.Nile)
	if err != nil {
		return err
	}
	res := p.run.Run(ctx, bin, engine.NileLogout(), engine.RunOptions{
		Env: engine.EnvFor(engine.Nile, p.paths),
	})
	if res.Err != nil {
		return fmt.Errorf("nile logout failed: %s", common.FirstLines(res.Stderr, 5))
	}
	return nil
}
