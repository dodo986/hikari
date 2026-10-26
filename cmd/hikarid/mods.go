package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/library"
	"hikari/internal/mods"
)

type modsGameView struct {
	mods.Game
	Store  string `json:"store,omitempty"`
	GameID string `json:"game_id,omitempty"`
}

// refreshModsSettings applies a settings.toml edit without a daemon restart.
func (d *daemon) refreshModsSettings() {
	info, err := os.Stat(d.paths.Settings)
	if err != nil {
		return
	}
	d.modsMu.Lock()
	stale := !info.ModTime().Equal(d.modsSettingsAt)
	d.modsMu.Unlock()
	if !stale {
		return
	}
	settings, err := config.LoadSettings(d.paths)
	if err != nil {
		d.log.Warn("mods: settings.toml could not be re-read", "err", err)
		return
	}
	if bridge, ok := d.mods.(*mods.Bridge); ok {
		bridge.Configure(settings.Mods.SourceDir, settings.Mods.Python, settings.Mods.Filegraph)
	}
	d.modsMu.Lock()
	d.modsSettingsAt = info.ModTime()
	d.modsMu.Unlock()
}

func (d *daemon) registerMods(srv *api.Server) {
	load := func() { d.refreshModsSettings() }
	srv.Register("mods.status", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		return d.mods.Status(c), nil
	})
	srv.Register("mods.games", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		games, err := d.mods.Games(c)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"games": d.annotateModsGames(c, games)}, nil
	})
	srv.Register("mods.setup", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		return d.startModsJob(c, srv, "", "setting up", func(ctx context.Context, sink mods.Sink) (string, error) {
			return "", d.mods.Setup(ctx, sink)
		})
	})
	srv.Register("mods.deploy", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, profile?}")
		}
		load()
		return d.startModsJob(c, srv, p.Game, "deploying", func(ctx context.Context, sink mods.Sink) (string, error) {
			return "", d.mods.Deploy(ctx, p.Game, p.Profile, sink)
		})
	})
	srv.Register("mods.restore", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game string `json:"game"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game}")
		}
		load()
		return d.startModsJob(c, srv, p.Game, "restoring", func(ctx context.Context, sink mods.Sink) (string, error) {
			return "", d.mods.Restore(ctx, p.Game, sink)
		})
	})
	srv.Register("mods.nexus", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Action string `json:"action"`
			Key    string `json:"key"`
		}
		_ = json.Unmarshal(params, &p)
		load()
		stored, err := d.mods.Nexus(c, p.Action, p.Key)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"stored": stored}, nil
	})
	srv.Register("mods.cancel", func(c context.Context, params json.RawMessage) (any, error) {
		d.modsMu.Lock()
		cancel := d.modsCancel
		d.modsMu.Unlock()
		if cancel == nil {
			return nil, api.Errorf(api.CodeNotFound, "no mods job is running")
		}
		cancel()
		return map[string]any{"canceled": true}, nil
	})
	srv.Register("mods.account", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		return d.mods.Account(c)
	})
	srv.Register("mods.login", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		return d.startModsJob(c, srv, "", "signing in", func(ctx context.Context, sink mods.Sink) (string, error) {
			_, err := d.mods.Login(ctx, sink)
			return "", err
		})
	})
	srv.Register("mods.logout", func(c context.Context, params json.RawMessage) (any, error) {
		load()
		if err := d.mods.Logout(c); err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"stored": false}, nil
	})

	srv.Register("mods.browse", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game  string `json:"game"`
			Query string `json:"query"`
			Sort  string `json:"sort"`
			Page  int    `json:"page"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, query?, sort?, page?}")
		}
		load()
		return d.mods.Browse(c, p.Game, p.Query, p.Sort, p.Page)
	})

	srv.Register("mods.files", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game  string `json:"game"`
			ModID int    `json:"mod_id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" || p.ModID <= 0 {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, mod_id}")
		}
		load()
		mod, files, err := d.mods.Files(c, p.Game, p.ModID)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"mod": mod, "files": files}, nil
	})

	srv.Register("mods.toggle", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" || p.Name == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, name, profile?}")
		}
		load()
		enabled, err := d.mods.Toggle(c, p.Game, p.Profile, p.Name)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"name": p.Name, "enabled": enabled}, nil
	})
	srv.Register("mods.remove", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" || p.Name == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, name, profile?}")
		}
		load()
		return d.startModsJob(c, srv, p.Game, "removing", func(ctx context.Context, sink mods.Sink) (string, error) {
			return "", d.mods.Remove(ctx, p.Game, p.Profile, p.Name, sink)
		})
	})
	srv.Register("mods.missing", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, profile?}")
		}
		load()
		missing, err := d.mods.Missing(c, p.Game, p.Profile)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"missing": missing}, nil
	})
	// A free account installs from the browser; this says when it landed.
	srv.Register("mods.downloads", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game   string `json:"game"`
			ModID  int    `json:"mod_id"`
			FileID int    `json:"file_id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" || p.ModID <= 0 {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, mod_id, file_id?}")
		}
		load()
		found, err := d.mods.Downloads(c, p.Game, p.ModID, p.FileID)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"found": found}, nil
	})
	srv.Register("mods.installed", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, profile?}")
		}
		load()
		return d.mods.Installed(c, p.Game, p.Profile)
	})

	srv.Register("mods.install", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
			Archive string `json:"archive"`
			URL     string `json:"url"`
			Name    string `json:"name"`
			ModID   int    `json:"mod_id"`
			FileID  int    `json:"file_id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, archive|url|mod_id+file_id, profile?, name?}")
		}
		phase := "installing"
		if p.URL != "" || p.ModID > 0 {
			phase = "downloading"
		}
		load()
		return d.startModsJob(c, srv, p.Game, phase, func(ctx context.Context, sink mods.Sink) (string, error) {
			return d.mods.Install(ctx, mods.InstallRequest{
				Game: p.Game, Profile: p.Profile, Archive: p.Archive, URL: p.URL,
				Name: p.Name, ModID: p.ModID, FileID: p.FileID,
			}, sink)
		})
	})
}

func (d *daemon) startModsJob(
	c context.Context, srv *api.Server, game, phase string,
	work func(ctx context.Context, sink mods.Sink) (string, error),
) (any, error) {
	d.modsMu.Lock()
	if running := d.modsJob; running != "" {
		d.modsMu.Unlock()
		return nil, api.Errorf(api.CodeBusy, "a mods job (%s) is already running", running)
	}
	ctx, cancel := context.WithCancel(c)
	job := fmt.Sprintf("mods-%d", time.Now().UnixNano())
	d.modsJob = phase
	d.modsCancel = cancel
	d.modsMu.Unlock()

	go func() {
		defer func() {
			cancel()
			d.modsMu.Lock()
			d.modsJob = ""
			d.modsCancel = nil
			d.modsMu.Unlock()
		}()
		sink := mods.Sink{
			Log: func(line string) {
				srv.Publish("mods.log", map[string]any{"job": job, "game": game, "line": line})
			},
			AuthURL: func(url string) {
				srv.Publish("mods.auth_url", map[string]any{"job": job, "url": url})
			},
			Progress: func(p mods.Progress) {
				srv.Publish("mods.progress", map[string]any{
					"job": job, "game": game, "phase": phase,
					"current": p.Current, "total": p.Total, "message": p.Message,
				})
			},
		}
		mod, err := work(ctx, sink)
		event := map[string]any{"job": job, "game": game, "ok": err == nil}
		if mod != "" {
			event["mod"] = mod
		}
		if err != nil {
			event["error"] = modsJobError(err)
			d.log.Error("mods job failed", "phase", phase, "game", game, "err", err)
		}
		srv.Publish("mods.done", event)
	}()
	return map[string]any{"job": job}, nil
}

func (d *daemon) annotateModsGames(ctx context.Context, games []mods.Game) []modsGameView {
	known, err := d.library.List(ctx, "", true)
	if err != nil {
		d.log.Warn("mods: cannot match games against the library", "err", err)
	}
	views := make([]modsGameView, 0, len(games))
	for _, game := range games {
		view := modsGameView{Game: game}
		if match, ok := matchLibraryGame(known, game); ok {
			view.Store = match.Store
			view.GameID = match.ID
		}
		views = append(views, view)
	}
	return views
}

// Amethyst's handler and Hikari's library entry have no shared id, so the join
// is Steam app id first, then install path, then title.
func matchLibraryGame(known []library.Game, target mods.Game) (library.Game, bool) {
	installed := make([]library.Game, 0, len(known))
	for _, game := range known {
		if game.Installed {
			installed = append(installed, game)
		}
	}
	if target.SteamAppID != "" {
		for _, game := range installed {
			if game.Store == library.StoreSteam && game.ID == target.SteamAppID {
				return game, true
			}
		}
	}
	if target.Path != "" {
		wanted := filepath.Clean(target.Path)
		for _, game := range installed {
			if game.InstallPath != "" && filepath.Clean(game.InstallPath) == wanted {
				return game, true
			}
		}
	}
	for _, game := range installed {
		if strings.EqualFold(game.Title, target.Name) {
			return game, true
		}
	}
	return library.Game{}, false
}

func modsJobError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "the job was cancelled"
	}
	return err.Error()
}

func modsError(err error) error {
	switch mods.KindOf(err) {
	case mods.KindUsage:
		return api.Errorf(api.CodeInvalidParams, "%s", err)
	case mods.KindNotFound:
		return api.Errorf(api.CodeNotFound, "%s", err)
	case mods.KindUnsupported:
		return api.Errorf(api.CodeUnsupported, "%s", err)
	case mods.KindConfig, mods.KindUnavailable, mods.KindUnconfigured:
		return api.Errorf(api.CodeNotAvailable, "%s", err)
	}
	if errors.Is(err, context.Canceled) {
		return api.Errorf(api.CodeNotAvailable, "the mods job was cancelled")
	}
	return api.Errorf(api.CodeInternal, "%s", err)
}
