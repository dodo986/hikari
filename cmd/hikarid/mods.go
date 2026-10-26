package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"hikari/internal/api"
	"hikari/internal/library"
	"hikari/internal/mods"
)

type modsGameView struct {
	mods.Game
	Store  string `json:"store,omitempty"`
	GameID string `json:"game_id,omitempty"`
}

func (d *daemon) registerMods(srv *api.Server) {
	srv.Register("mods.status", func(c context.Context, params json.RawMessage) (any, error) {
		return d.mods.Status(c), nil
	})
	srv.Register("mods.games", func(c context.Context, params json.RawMessage) (any, error) {
		games, err := d.mods.Games(c)
		if err != nil {
			return nil, modsError(err)
		}
		return map[string]any{"games": d.annotateModsGames(c, games)}, nil
	})
	srv.Register("mods.setup", func(c context.Context, params json.RawMessage) (any, error) {
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
		return d.startModsJob(c, srv, p.Game, "restoring", func(ctx context.Context, sink mods.Sink) (string, error) {
			return "", d.mods.Restore(ctx, p.Game, sink)
		})
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
	srv.Register("mods.install", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Game    string `json:"game"`
			Profile string `json:"profile"`
			Archive string `json:"archive"`
			URL     string `json:"url"`
			Name    string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Game == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {game, archive|url, profile?, name?}")
		}
		phase := "installing"
		if p.URL != "" {
			phase = "downloading"
		}
		return d.startModsJob(c, srv, p.Game, phase, func(ctx context.Context, sink mods.Sink) (string, error) {
			return d.mods.Install(ctx, mods.InstallRequest{
				Game: p.Game, Profile: p.Profile, Archive: p.Archive, URL: p.URL, Name: p.Name,
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
