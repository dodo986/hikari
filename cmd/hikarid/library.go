package main

import (
	"context"
	"encoding/json"
	"hikari/internal/api"
	"hikari/internal/art"
	"hikari/internal/config"
	"hikari/internal/library"
	"strings"
	"time"
)

func (d *daemon) registerLibrary(srv *api.Server) {
	srv.Register("library.list", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store         string `json:"store"`
			InstalledOnly bool   `json:"installed_only"`
		}
		if len(params) > 0 {
			if err := json.Unmarshal(params, &p); err != nil {
				return nil, api.Errorf(api.CodeInvalidParams, "params must be {store?, installed_only?}")
			}
		}
		games, err := d.library.List(c, p.Store, p.InstalledOnly)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		if games == nil {
			games = []library.Game{}
		}
		d.decorateArtwork(games)
		d.decorateState(games)
		go d.fetchMissingArtwork(srv, games)
		return map[string]any{"games": games}, nil
	})
	srv.Register("library.get", func(c context.Context, params json.RawMessage) (any, error) {
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
		games := []library.Game{g}
		d.decorateArtwork(games)
		d.decorateState(games)
		g = games[0]
		go d.fetchMissingArtwork(srv, games)
		return g, nil
	})
}

func (d *daemon) decorateState(games []library.Game) {
	for i := range games {
		e := d.state.Entry(games[i].Store, games[i].ID)
		if e == nil {
			continue
		}
		if e.Favorite {
			games[i].Favorite = true
		}
		if !e.LastPlayed.IsZero() {
			at := e.LastPlayed
			games[i].LastPlayed = &at
		}
		games[i].PlayCount = e.PlayCount
	}
}

func (d *daemon) decorateArtwork(games []library.Game) {
	for i := range games {
		g := &games[i]
		if e := d.state.Entry(g.Store, g.ID); e != nil && e.Artwork != nil {
			applyOverride(&g.Artwork, *e.Artwork)
		}
		local := library.Artwork{
			Cover:  d.cachedPath(g.Artwork.Cover),
			Square: d.cachedPath(g.Artwork.Square),
			Logo:   d.cachedPath(g.Artwork.Logo),
			Icon:   d.cachedPath(g.Artwork.Icon),
		}
		if local != (library.Artwork{}) {
			g.ArtworkLocal = local
		}
	}
}

func (d *daemon) cachedPath(url string) string {
	if url == "" {
		return ""
	}
	p := d.art.PathFor(url)
	if d.art.Has(url) {
		return p
	}
	return ""
}

// reResolveArtwork forgets what it tried and looks the whole library up again,
// which is what a newly entered SteamGridDB key should trigger.
func (d *daemon) reResolveArtwork(srv *api.Server) {
	games, err := d.library.List(context.Background(), "", false)
	if err != nil {
		d.log.Warn("cannot relist the library for artwork", "err", err)
		return
	}
	d.fetchMu.Lock()
	d.artTried = map[string]bool{}
	d.artShape = map[string]bool{}
	d.fetchMu.Unlock()
	d.decorateArtwork(games)
	go d.fetchMissingArtwork(srv, games)
}

func (d *daemon) landscapeCover(g library.Game) bool {
	p := d.art.PathFor(g.Artwork.Cover)
	if p == "" || !d.art.Has(g.Artwork.Cover) {
		return false
	}
	d.fetchMu.Lock()
	known, seen := d.artShape[p]
	d.fetchMu.Unlock()
	if seen {
		return known
	}
	w, h, err := art.Dimensions(p)
	wide := err == nil && w > h
	d.fetchMu.Lock()
	d.artShape[p] = wide
	d.fetchMu.Unlock()
	return wide
}

// withoutRetina drops the 2x suffix, since plenty of older app ids only publish
// the plain grid.
func withoutRetina(url string) string {
	return strings.Replace(url, "_2x.jpg", ".jpg", 1)
}

// applyOverride lets stored artwork win over what a store reports, which is how
// an upgraded portrait grid survives a library rescan.
func applyOverride(into *library.Artwork, o config.ArtworkOverride) {
	if o.Cover != "" {
		into.Cover = o.Cover
	}
	if o.Square != "" {
		into.Square = o.Square
	}
	if o.Logo != "" {
		into.Logo = o.Logo
	}
	if o.Icon != "" {
		into.Icon = o.Icon
	}
}

func (d *daemon) fetchMissingArtwork(srv *api.Server, games []library.Game) {
	var todo []library.Game
	for _, g := range games {
		wanted := g.Artwork.Cover == "" || (g.Artwork.Square == "" && g.Artwork.Icon == "")
		if !wanted && !anyMissing(d.art, g.Artwork) && !d.landscapeCover(g) {
			continue
		}
		d.fetchMu.Lock()
		if !d.fetchPending[g.Key()] {
			d.fetchPending[g.Key()] = true
			todo = append(todo, g)
		}
		d.fetchMu.Unlock()
	}
	if len(todo) == 0 {
		return
	}
	fetchedStores := map[string]bool{}
	for _, g := range todo {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		ask := g
		if d.landscapeCover(g) {
			ask.Artwork.Cover = ""
		}
		resolved := ask.Artwork
		d.fetchMu.Lock()
		tried := d.artTried[g.Key()]
		d.artTried[g.Key()] = true
		d.fetchMu.Unlock()
		if !tried {
			got, err := d.resolveArt(ctx, ask)
			if err != nil {
				d.log.Debug("artwork not resolved", "game", g.Key(), "err", err)
			}
			resolved = got
		}
		keep := library.Artwork{}
		anyFetched := false
		for _, slot := range []struct {
			url string
			set func(*library.Artwork, string)
		}{
			{resolved.Cover, func(a *library.Artwork, u string) { a.Cover = u }},
			{resolved.Square, func(a *library.Artwork, u string) { a.Square = u }},
			{resolved.Logo, func(a *library.Artwork, u string) { a.Logo = u }},
			{resolved.Icon, func(a *library.Artwork, u string) { a.Icon = u }},
		} {
			if slot.url == "" {
				continue
			}
			if !d.art.Has(slot.url) {
				if _, err := d.art.Fetch(ctx, slot.url); err != nil {
					d.log.Debug("artwork fetch failed", "game", g.Key(), "url", slot.url, "err", err)
					if smaller := withoutRetina(slot.url); smaller != slot.url {
						if _, err := d.art.Fetch(ctx, smaller); err == nil {
							slot.url = smaller
							anyFetched = true
						}
					}
					if !d.art.Has(slot.url) {
						continue
					}
				}
				anyFetched = true
			}
			slot.set(&keep, slot.url)
		}
		cancel()
		d.state.SetArtwork(g.Store, g.ID, config.ArtworkOverride(keep))
		d.fetchMu.Lock()
		delete(d.fetchPending, g.Key())
		d.fetchMu.Unlock()
		if anyFetched {
			fetchedStores[g.Store] = true
		}
	}
	for store := range fetchedStores {
		srv.Publish("library.changed", map[string]any{"store": store})
	}
}

func anyMissing(c *art.Cache, a library.Artwork) bool {
	for _, u := range []string{a.Cover, a.Square, a.Logo, a.Icon} {
		if u != "" && !c.Has(u) {
			return true
		}
	}
	return false
}
