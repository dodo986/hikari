package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hikari/internal/api"
	"hikari/internal/winemgr"
	"time"
)

func (d *daemon) registerWine(srv *api.Server) {
	srv.Register("wine.versions", func(c context.Context, params json.RawMessage) (any, error) {
		versions, err := d.wine.List(c)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if versions == nil {
			versions = []winemgr.Version{}
		}
		return map[string]any{"versions": versions}, nil
	})
	srv.Register("wine.releases", func(c context.Context, params json.RawMessage) (any, error) {
		releases, err := d.wine.Releases(c)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if releases == nil {
			releases = []winemgr.Release{}
		}
		return map[string]any{"releases": releases}, nil
	})
	srv.Register("wine.download", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Tag == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {tag}")
		}
		job := fmt.Sprintf("wine-%d", time.Now().UnixNano())
		go func() {
			name, err := d.wine.Download(context.Background(), p.Tag, func(pr winemgr.DownloadProgress) {
				ev := map[string]any{"phase": pr.Phase, "done": pr.Done, "name": pr.Name}
				if pr.Percent != nil {
					ev["percent"] = *pr.Percent
				}
				srv.Publish("wine.progress", ev)
			})
			if err != nil {
				d.log.Error("wine download failed", "tag", p.Tag, "err", err)
				srv.Publish("wine.progress", map[string]any{"phase": "error", "done": true, "name": p.Tag, "error": err.Error()})
				return
			}
			d.log.Info("wine installed", "name", name)
		}()
		return map[string]any{"job": job, "tag": p.Tag}, nil
	})
	srv.Register("wine.remove", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {name}")
		}
		if err := d.wine.Remove(c, p.Name); err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return map[string]any{}, nil
	})
}
