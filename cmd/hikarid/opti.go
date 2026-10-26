package main

import (
	"context"
	"fmt"
	"time"

	"encoding/json"
	"hikari/internal/api"
	"hikari/internal/library"
	"hikari/internal/opti"
	steampkg "hikari/internal/providers/steam"
)

func (d *daemon) registerOpti(srv *api.Server) {
	publishProgress := func(store, id string, p opti.Progress) {
		ev := map[string]any{"phase": p.Phase, "done": p.Done}
		if p.Percent != nil {
			ev["percent"] = *p.Percent
		}
		if store != "" {
			ev["store"] = store
			ev["id"] = id
		}
		srv.Publish("opti.progress", ev)
	}

	srv.Register("opti.detect", func(c context.Context, params json.RawMessage) (any, error) {
		gpus, err := d.opti.DetectGPUs(c)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return map[string]any{"gpus": gpus}, nil
	})
	srv.Register("opti.releases", func(c context.Context, params json.RawMessage) (any, error) {
		releases, err := d.opti.ListReleases(c)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return map[string]any{"releases": releases}, nil
	})
	srv.Register("opti.versions", func(c context.Context, params json.RawMessage) (any, error) {
		v, err := d.opti.ListVersions(c)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if v == nil {
			v = []string{}
		}
		return map[string]any{"versions": v}, nil
	})

	statusOf := func(c context.Context, params json.RawMessage) (opti.Status, library.Game, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return opti.Status{}, library.Game{}, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return opti.Status{}, g, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		st, err := d.opti.Status(c, p.ID, d.gameExePath(g), g.InstallPath)
		if err != nil || len(st) == 0 {
			return opti.Status{GameID: p.ID}, g, err
		}
		return st[0], g, nil
	}

	srv.Register("opti.download", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Tag == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {tag}")
		}
		job := fmt.Sprintf("opti-download-%d", time.Now().UnixNano())
		go func() {
			err := d.opti.Download(context.Background(), p.Tag, func(pr opti.Progress) {
				publishProgress("", "", pr)
			})
			if err != nil {
				d.log.Error("optiscaler download failed", "tag", p.Tag, "err", err)
				publishProgress("", "", opti.Progress{Phase: "error", Done: true})
				return
			}
		}()
		return map[string]any{"job": job, "tag": p.Tag}, nil
	})

	srv.Register("opti.status", func(c context.Context, params json.RawMessage) (any, error) {
		st, g, err := statusOf(c, params)
		if err != nil {
			return nil, err
		}
		if st.Installed && g.Runner == library.RunnerSteam {
			st.LaunchOptions = steampkg.LaunchOptions(st.InjectionDLL)
		}
		return st, nil
	})

	srv.Register("opti.install", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store     string `json:"store"`
			ID        string `json:"id"`
			Tag       string `json:"tag"`
			Injection string `json:"injection"`
			Upscaler  string `json:"upscaler"`
			FrameGen  bool   `json:"framegen"`
			FGInput   string `json:"fg_input"`
			FGOutput  string `json:"fg_output"`
			SpoofDLSS *bool  `json:"spoof_dlss"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id, ...}")
		}
		g, err := d.library.Get(c, p.Store, p.ID)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		if !g.Installed {
			return nil, api.Errorf(api.CodeUnsupported, "%s is not installed", g.Title)
		}
		tag := p.Tag
		if tag == "" {
			releases, err := d.opti.ListReleases(c)
			if err != nil || len(releases) == 0 {
				return nil, api.Errorf(api.CodeNotAvailable, "no OptiScaler release to install (offline? pass a downloaded tag)")
			}
			tag = releases[0].Tag
		}

		opts := opti.InstallOptions{
			GameID: p.ID, InstallDir: g.InstallPath, ExePath: d.gameExePath(g),
			Version: tag, Injection: p.Injection, Upscaler: p.Upscaler,
			FrameGen: p.FrameGen, FGInput: p.FGInput, FGOutput: p.FGOutput,
		}
		opts.SpoofDLSS = opti.DefaultSpoof(opti.GPU{})
		if p.SpoofDLSS != nil {
			opts.SpoofDLSS = *p.SpoofDLSS
		} else if gpus, err := d.opti.DetectGPUs(c); err == nil && len(gpus) > 0 {
			opts.SpoofDLSS = opti.DefaultSpoof(gpus[0])
		}

		job := fmt.Sprintf("opti-%d", time.Now().UnixNano())
		go func() {
			err := d.opti.Install(context.Background(), opts, func(pr opti.Progress) {
				publishProgress(p.Store, p.ID, pr)
			})
			if err != nil {
				d.log.Error("optiscaler install failed", "game", g.Key(), "err", err)
				done := true
				publishProgress(p.Store, p.ID, opti.Progress{Phase: "error", Done: done})
				return
			}
			srv.Publish("opti.changed", map[string]any{"store": p.Store, "id": p.ID, "installed": true})
		}()
		return map[string]any{"job": job, "tag": tag}, nil
	})

	srv.Register("opti.uninstall", func(c context.Context, params json.RawMessage) (any, error) {
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
		if err := d.opti.Uninstall(c, p.ID, d.gameExePath(g), g.InstallPath); err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		srv.Publish("opti.changed", map[string]any{"store": p.Store, "id": p.ID, "installed": false})
		return map[string]any{}, nil
	})
}
