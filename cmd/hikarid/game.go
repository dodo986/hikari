package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/library"
	"time"
)

func (d *daemon) registerGame(srv *api.Server) {
	installerFor := func(store string) (library.Installer, library.Provider, error) {
		p := d.library.Provider(store)
		if p == nil {
			return nil, nil, api.Errorf(api.CodeInvalidParams, "unknown store: %s", store)
		}
		inst, ok := p.(library.Installer)
		if !ok {
			return nil, p, api.Errorf(api.CodeUnsupported, "store %s cannot install games", store)
		}
		return inst, p, nil
	}

	srv.Register("game.install", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			library.InstallOptions
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" || p.Path == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, path, ...}")
		}
		inst, prov, err := installerFor(p.Store)
		if err != nil {
			return nil, err
		}
		if existing, err := d.library.Get(c, p.Store, p.ID); err == nil && existing.Source == library.SourceHeroicImport {
			return nil, api.Errorf(api.CodeUnsupported,
				"%s is managed by the Heroic launcher; Hikari only launches imported games", existing.Title)
		}
		key := p.Store + "/" + p.ID
		if !d.beginJob(key) {
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		job := fmt.Sprintf("install-%d", time.Now().UnixNano())
		instCtx, cancel := context.WithCancel(context.Background())
		if !d.ownJob(key, &installJob{id: job, cancel: cancel}) {
			cancel()
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		go func() {
			defer d.endJob(key)
			err := inst.Install(instCtx, p.ID, p.InstallOptions, func(pr library.InstallProgress) {
				srv.Publish("install.progress", map[string]any{
					"store": p.Store, "id": p.ID, "state": pr.State,
					"percent": pr.Percent, "bytes": pr.Bytes, "eta": pr.ETA,
					"down_speed": pr.DownSpeed, "disk_speed": pr.DiskSpeed,
				})
			})
			if err != nil {
				if errors.Is(err, library.ErrInstallCanceled) {
					d.log.Info("install canceled", "game", key)
				} else {
					d.log.Error("install failed", "game", key, "err", err)
				}
				return
			}
			d.markInstalled(p.Store, p.ID, false)
			srv.Publish("library.changed", map[string]any{"store": p.Store, "updated": []string{p.ID}})
		}()
		_ = prov
		return map[string]any{"job": job, "state": "downloading"}, nil
	})

	srv.Register("game.install_info", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store    string `json:"store"`
			ID       string `json:"id"`
			Platform string `json:"platform"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, platform?}")
		}
		prov := d.library.Provider(p.Store)
		if prov == nil {
			return nil, api.Errorf(api.CodeInvalidParams, "unknown store: %s", p.Store)
		}
		ii, ok := prov.(library.InstallInfoProvider)
		if !ok {
			return nil, api.Errorf(api.CodeUnsupported, "store %s cannot describe installs", p.Store)
		}
		info, err := ii.InstallInfo(c, p.ID, p.Platform)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return info, nil
	})

	srv.Register("game.update", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			library.InstallOptions
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, ...}")
		}
		prov := d.library.Provider(p.Store)
		if prov == nil {
			return nil, api.Errorf(api.CodeInvalidParams, "unknown store: %s", p.Store)
		}
		up, ok := prov.(library.Updater)
		if !ok {
			return nil, api.Errorf(api.CodeUnsupported, "store %s cannot update games", p.Store)
		}
		if g, err := d.library.Get(c, p.Store, p.ID); err == nil && !g.Installed {
			return nil, api.Errorf(api.CodeUnsupported, "%s is not installed", g.Title)
		}
		key := p.Store + "/" + p.ID
		if !d.beginJob(key) {
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		job := fmt.Sprintf("update-%d", time.Now().UnixNano())
		upCtx, cancel := context.WithCancel(context.Background())
		if !d.ownJob(key, &installJob{id: job, cancel: cancel}) {
			cancel()
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		go func() {
			defer d.endJob(key)
			err := up.Update(upCtx, p.ID, p.InstallOptions, func(pr library.InstallProgress) {
				srv.Publish("install.progress", map[string]any{
					"store": p.Store, "id": p.ID, "state": pr.State,
					"percent": pr.Percent, "bytes": pr.Bytes, "eta": pr.ETA,
					"down_speed": pr.DownSpeed, "disk_speed": pr.DiskSpeed,
				})
			})
			if err != nil {
				if errors.Is(err, library.ErrInstallCanceled) {
					d.log.Info("update canceled", "game", key)
				} else {
					d.log.Error("update failed", "game", key, "err", err)
				}
				return
			}
			srv.Publish("library.changed", map[string]any{"store": p.Store, "updated": []string{p.ID}})
		}()
		return map[string]any{"job": job, "state": "updating"}, nil
	})

	srv.Register("game.favorite", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store    string `json:"store"`
			ID       string `json:"id"`
			Favorite *bool  `json:"favorite"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, favorite?}")
		}
		var favorite bool
		if err := d.state.UpdateEntry(p.Store, p.ID, func(e *config.GameEntry) {
			if p.Favorite != nil {
				e.Favorite = *p.Favorite
			} else {
				e.Favorite = !e.Favorite
			}
			favorite = e.Favorite
		}); err != nil {
			return nil, err
		}
		srv.Publish("library.changed", map[string]any{"store": p.Store, "updated": []string{p.ID}})
		return map[string]any{"favorite": favorite}, nil
	})

	srv.Register("game.launch_options", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		prov := d.library.Provider(p.Store)
		lp, ok := prov.(library.LaunchOptionProvider)
		if !ok {
			return map[string]any{"options": []library.LaunchOption{}}, nil
		}
		options, err := lp.LaunchOptions(c, g)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if options == nil {
			options = []library.LaunchOption{}
		}
		return map[string]any{"options": options}, nil
	})

	srv.Register("game.repair", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			library.InstallOptions
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		prov := d.library.Provider(p.Store)
		rp, ok := prov.(library.Repairer)
		if !ok {
			return nil, api.Errorf(api.CodeUnsupported, "store %s cannot repair games", p.Store)
		}
		if g, err := d.library.Get(c, p.Store, p.ID); err == nil && !g.Installed {
			return nil, api.Errorf(api.CodeUnsupported, "%s is not installed", g.Title)
		}
		key := p.Store + "/" + p.ID
		if !d.beginJob(key) {
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		job := fmt.Sprintf("repair-%d", time.Now().UnixNano())
		rctx, cancel := context.WithCancel(context.Background())
		if !d.ownJob(key, &installJob{id: job, cancel: cancel}) {
			cancel()
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		go func() {
			defer d.endJob(key)
			err := rp.Repair(rctx, p.ID, p.InstallOptions, func(pr library.InstallProgress) {
				srv.Publish("install.progress", map[string]any{
					"store": p.Store, "id": p.ID, "state": pr.State,
					"percent": pr.Percent, "bytes": pr.Bytes, "eta": pr.ETA,
				})
			})
			if err != nil {
				if errors.Is(err, library.ErrInstallCanceled) {
					d.log.Info("repair canceled", "game", key)
				} else {
					d.log.Error("repair failed", "game", key, "err", err)
				}
			}
		}()
		return map[string]any{"job": job, "state": "repairing"}, nil
	})

	srv.Register("game.saves.sync", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store     string `json:"store"`
			ID        string `json:"id"`
			Direction string `json:"direction"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, direction?}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		prov := d.library.Provider(p.Store)
		ss, ok := prov.(library.SaveSyncer)
		if !ok {
			return nil, api.Errorf(api.CodeUnsupported, "store %s cannot sync saves yet", p.Store)
		}
		settings := d.settingsFor(g)
		out, err := ss.SyncSaves(c, p.ID, library.SaveOptions{
			Direction: p.Direction, SavePath: settings.SavesPath,
		})
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return map[string]any{"output": out}, nil
	})

	srv.Register("game.install.cancel", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		d.jobsMu.Lock()
		job := d.jobs[p.Store+"/"+p.ID]
		d.jobsMu.Unlock()
		if job == nil {
			return nil, api.Errorf(api.CodeBusy, "no install is running for %s/%s", p.Store, p.ID)
		}
		job.cancel()
		return map[string]any{"state": "canceled"}, nil
	})

	srv.Register("game.uninstall", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store     string `json:"store"`
			ID        string `json:"id"`
			KeepFiles bool   `json:"keep_files"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, keep_files?}")
		}
		inst, _, err := installerFor(p.Store)
		if err != nil {
			return nil, err
		}
		if existing, err := d.library.Get(c, p.Store, p.ID); err == nil && existing.Source == library.SourceHeroicImport {
			return nil, api.Errorf(api.CodeUnsupported,
				"%s is managed by the Heroic launcher; uninstall it there", existing.Title)
		}
		key := p.Store + "/" + p.ID
		if !d.beginJob(key) {
			return nil, api.Errorf(api.CodeBusy, "an operation is already running for %s", key)
		}
		defer d.endJob(key)
		if err := inst.Uninstall(c, p.ID, p.KeepFiles); err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		d.markInstalled(p.Store, p.ID, true)
		srv.Publish("library.changed", map[string]any{"store": p.Store, "removed": []string{p.ID}})
		return map[string]any{"state": "done"}, nil
	})
}
