package main

import (
	"context"
	"encoding/json"
	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/launch"
	"hikari/internal/library"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (d *daemon) registerLaunch(srv *api.Server) {
	srv.Register("game.launch", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store  string                `json:"store"`
			ID     string                `json:"id"`
			Args   []string              `json:"args"`
			DryRun bool                  `json:"dry_run"`
			Option *library.LaunchOption `json:"option"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, args?, dry_run?}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		if !g.Installed {
			return nil, api.Errorf(api.CodeUnsupported, "%s is not installed", g.Title)
		}
		if d.demo != nil && g.Source == library.SourceDemo {
			if p.DryRun {
				return nil, api.Errorf(api.CodeUnsupported, "demo games have no real launch command")
			}
			life := 90 * time.Second
			if dl, ok := d.library.Provider(g.Store).(library.DemoLauncher); ok {
				life = dl.DemoLaunchLife(g)
			}
			if err := d.games.LaunchFake(c, g.Store, g.ID, life, func(state string, pid int) {
				event := map[string]any{"store": g.Store, "id": g.ID, "state": state}
				if pid != 0 {
					event["pid"] = pid
				}
				srv.Publish("game.state", event)
			}); err != nil {
				return nil, api.Errorf(api.CodeBusy, "%s", err)
			}
			_ = d.state.UpdateEntry(g.Store, g.ID, func(e *config.GameEntry) {
				e.PlayCount++
				e.LastPlayed = time.Now()
			})
			return map[string]any{"state": "playing"}, nil
		}

		if g.Runner == library.RunnerSteam {
			if p.DryRun {
				return map[string]any{"plan": &launch.Plan{
					Argv:   []string{"steam", "-applaunch", g.ID},
					Env:    map[string]string{},
					Engine: "steam", WineType: launch.WineNative,
				}}, nil
			}
			bin, err := exec.LookPath("steam")
			if err != nil {
				return nil, api.Errorf(api.CodeNotAvailable, "steam not found; install Steam to launch Steam games")
			}
			plan := &launch.Plan{
				Argv: []string{bin, "-applaunch", g.ID},
				Env:  launch.WrapperEnv(g.Store, g.ID), Engine: "steam", WineType: launch.WineNative,
			}
			if err := d.games.Launch(c, g.Store, g.ID, plan, func(state string, pid int) {
				event := map[string]any{"store": g.Store, "id": g.ID, "state": state}
				if pid != 0 {
					event["pid"] = pid
				}
				srv.Publish("game.state", event)
			}); err != nil {
				return nil, api.Errorf(api.CodeBusy, "%s", err)
			}
			return map[string]any{"state": "launching"}, nil
		}
		settings := d.settingsFor(g)
		launchOpts := launch.Options{Args: p.Args}
		if p.Option != nil {
			switch p.Option.Type {
			case "basic":
				launchOpts.Args = append(launchOpts.Args, p.Option.Params...)
			case "altExe":
				if p.Option.Exe != "" {
					settings.TargetExe = p.Option.Exe
				}
			case "dlc":
				if p.Option.DLCApp != "" && g.Runner == library.RunnerLegendary {
					launchOpts.AppIDOverride = p.Option.DLCApp
				}
			}
		}
		umuID := ""
		if !settings.DisableUMU {
			if id, err := d.umu.Resolve(c, g.Store, g.ID); err != nil {
				d.log.Debug("umu id unresolved, using umu-0", "game", g.Key(), "err", err)
			} else {
				umuID = id
			}
		}
		launchOpts.UmuID = umuID
		launchOpts.ModOverrides = d.modOverrides(c, g)
		plan, err := launch.Build(g, settings, d.paths, d.eng, launchOpts)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if g.Store == library.StoreGOG {
			if sc := d.cometSidecar(c); sc != nil {
				plan.Sidecars = append(plan.Sidecars, *sc)
			}
		}
		if p.DryRun {
			return map[string]any{"plan": plan}, nil
		}

		plan.Prepare = func(ctx context.Context) error {
			if err := launch.EnsurePrefix(ctx, d.run, d.eng, settings, plan.WineType, plan.Env); err != nil {
				return err
			}

			launch.PrepareExtras(ctx, d.run, d.eng, d.tools, d.log, settings, plan.WineType, plan.Env)
			launch.PrepareRuntimes(ctx, d.runtimes, d.log, settings, plan.WineType)
			launch.PrepareEOSOverlay(ctx, d.run, d.eng, d.log, settings, g,
				launch.EOSOverlayDir(d.paths.DataDir), plan.Env)
			return nil
		}
		if err := d.games.Launch(c, g.Store, g.ID, plan, func(state string, pid int) {
			event := map[string]any{"store": g.Store, "id": g.ID, "state": state}
			if pid != 0 {
				event["pid"] = pid
			}
			if state == launch.StateCrashed {
				if rec, ok := d.games.LastRun(g.Store, g.ID); ok {
					if rec.Error != "" {
						event["error"] = rec.Error
					}
					if len(rec.Lines) > 0 {
						event["log"] = lastLines(rec.Lines, 20)
					}
				}
			}
			srv.Publish("game.state", event)
			if state == launch.StatePlaying {
				_ = d.state.UpdateEntry(g.Store, g.ID, func(e *config.GameEntry) {
					e.PlayCount++
					e.LastPlayed = time.Now()
				})
			}
		}); err != nil {
			return nil, api.Errorf(api.CodeBusy, "%s", err)
		}
		return map[string]any{"state": "launching"}, nil
	})

	srv.Register("game.running", func(c context.Context, params json.RawMessage) (any, error) {
		games := d.games.Running()
		if games == nil {
			games = []launch.RunningGame{}
		}
		return map[string]any{"games": games}, nil
	})

	srv.Register("game.stop", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		sessionErr := d.games.Stop(p.Store + "/" + p.ID)
		var prefixErr error
		if g, err := d.library.Get(c, p.Store, p.ID); err == nil {
			settings := d.settingsFor(g)
			if settings.WinePrefix != "" && !strings.EqualFold(g.Platform, "Linux") {
				prefixErr = launch.StopPrefix(c, d.run, d.eng, settings)
			}
		}
		if sessionErr != nil && prefixErr != nil {
			return nil, api.Errorf(api.CodeUnsupported, "%s", sessionErr)
		}
		return map[string]any{"state": "stopped"}, nil
	})

	settingsGame := func(c context.Context, params json.RawMessage) (library.Game, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return library.Game{}, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		return d.library.Get(c, p.Store, p.ID)
	}

	srv.Register("game.settings.get", func(c context.Context, params json.RawMessage) (any, error) {
		g, err := settingsGame(c, params)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		return d.settingsFor(g), nil
	})

	srv.Register("game.settings.set", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store    string              `json:"store"`
			ID       string              `json:"id"`
			Settings config.GameSettings `json:"settings"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, settings}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}

		launch.DropDefaultPrefix(g, d.settings.Defaults, &p.Settings)
		if err := d.state.UpdateEntry(p.Store, p.ID, func(e *config.GameEntry) {
			e.Settings = p.Settings
			e.SettingsComplete = true
		}); err != nil {
			return nil, err
		}
		return d.settingsFor(g), nil
	})
}

func (d *daemon) cometSidecar(ctx context.Context) *launch.Sidecar {
	bin, err := d.eng.Require(engine.Comet)
	if err != nil {
		return nil
	}
	user := ""
	if prov := d.library.Provider(library.StoreGOG); prov != nil {
		if namer, ok := prov.(interface {
			Username(context.Context) (string, error)
		}); ok {
			if name, err := namer.Username(ctx); err == nil {
				user = name
			}
		}
	}
	if user == "" {
		d.log.Debug("comet skipped: no GOG username available")
		return nil
	}
	return &launch.Sidecar{
		Name: "comet",
		Argv: []string{bin, "--from-heroic", "--username", user, "--quit"},
	}
}

func (d *daemon) gameExePath(g library.Game) string {
	if g.Executable == "" {
		return ""
	}
	if filepath.IsAbs(g.Executable) {
		return g.Executable
	}
	if g.InstallPath == "" {
		return ""
	}
	return filepath.Join(g.InstallPath, g.Executable)
}

func lastLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
