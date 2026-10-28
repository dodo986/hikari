package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"hikari/internal/api"
	"hikari/internal/art"
	"hikari/internal/library"
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

// cachedPath is the local copy of a piece of artwork. Providers hand over either
// a remote URL or a file they found on disk, which is what Steam does with its
// librarycache, and a window cannot read the second kind: both are copied into
// the cache so one path shape reaches the clients.
func (d *daemon) cachedPath(source string) string {
	if source == "" {
		return ""
	}
	if isRemoteArt(source) {
		if d.art.Has(source) {
			return d.art.PathFor(source)
		}
		return ""
	}
	return d.seedLocalArt(source)
}

func (d *daemon) seedLocalArt(source string) string {
	info, err := os.Stat(source)
	if err != nil || info.IsDir() {
		return ""
	}
	file, err := os.Open(source)
	if err != nil {
		return ""
	}
	defer file.Close()
	path, err := d.art.SeedFile(source, file)
	if err != nil {
		d.log.Debug("local artwork could not be cached", "path", source, "err", err)
		return ""
	}
	return path
}

func (d *daemon) fetchMissingArtwork(srv *api.Server, games []library.Game) {
	var todo []library.Game
	for _, g := range games {
		if !anyMissing(d.art, g.Artwork) {
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
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		urls := []string{g.Artwork.Cover, g.Artwork.Square, g.Artwork.Logo, g.Artwork.Icon}
		anyFetched := false
		for _, u := range urls {
			if u == "" || d.art.Has(u) {
				continue
			}
			if _, err := d.art.Fetch(ctx, u); err != nil {
				d.log.Debug("artwork fetch failed", "game", g.Key(), "url", u, "err", err)
			} else {
				anyFetched = true
			}
		}
		cancel()
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

// anyMissing only asks about the artwork that has to travel over the network.
// A provider path is local by definition, so there is nothing to fetch for it.
func anyMissing(c *art.Cache, a library.Artwork) bool {
	for _, u := range []string{a.Cover, a.Square, a.Logo, a.Icon} {
		if isRemoteArt(u) && !c.Has(u) {
			return true
		}
	}
	return false
}

func isRemoteArt(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}
