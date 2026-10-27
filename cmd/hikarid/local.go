package main

import (
	"context"
	"encoding/json"

	"strings"

	"hikari/internal/api"
	"hikari/internal/artwork"
	"hikari/internal/config"
	"hikari/internal/library"
	"hikari/internal/providers/local"
)

func (d *daemon) registerLocal(srv *api.Server) {
	srv.Register("game.add_local", func(c context.Context, params json.RawMessage) (any, error) {
		var p local.Spec
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {title, executable, platform, args?, artwork?}")
		}
		g, err := d.local.Add(p)
		if err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
		}
		srv.Publish("library.changed", map[string]any{"store": library.StoreLocal, "added": []string{g.ID}})
		return g, nil
	})
	srv.Register("game.remove_local", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {id}")
		}
		if err := d.local.Remove(p.ID); err != nil {
			return nil, api.Errorf(api.CodeNotFound, "%s", err)
		}
		srv.Publish("library.changed", map[string]any{"store": library.StoreLocal, "removed": []string{p.ID}})
		return map[string]any{}, nil
	})
}

func (d *daemon) registerArtwork(srv *api.Server) {
	srv.Register("artwork.key", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {key}")
		}
		key := strings.TrimSpace(p.Key)
		if key != "" {
			if err := d.artwork.Check(c, key); err != nil {
				return nil, api.Errorf(api.CodeInvalidParams, "%s", err)
			}
		}
		d.artwork.SetKey(key)
		d.settings.Library.SteamGridDBKey = key
		if err := config.SaveSettings(d.paths, d.settings); err != nil {
			return nil, api.Errorf(api.CodeInternal, "%s", err)
		}
		d.reResolveArtwork(srv)
		return map[string]any{"stored": key != ""}, nil
	})
	srv.Register("artwork.search", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {title}")
		}
		cands, err := d.artwork.Search(c, p.Title)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if cands == nil {
			cands = []artwork.Candidate{}
		}
		return map[string]any{"candidates": cands}, nil
	})
}
