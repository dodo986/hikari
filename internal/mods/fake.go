package mods

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Fake is the demo daemon's backend: no Amethyst, no game writes.
type Fake struct {
	mu       sync.Mutex
	games    []Game
	deployed map[string]string
}

func NewFake() *Fake {
	return &Fake{
		games: []Game{
			{
				ID: "skyrim_se", Name: "Skyrim Special Edition", SteamAppID: "489830",
				Configured: true, Path: "/games/Skyrim Special Edition",
				Profiles: []string{"default", "vanilla-plus"},
			},
			{
				ID: "cyberpunk_2077", Name: "Cyberpunk 2077", SteamAppID: "1091500",
				Configured: true, Path: "/games/Cyberpunk 2077",
				Profiles: []string{"default"},
			},
			{
				ID: "baldurs_gate_3", Name: "Baldur's Gate 3", SteamAppID: "1086940",
				Configured: false, Profiles: []string{},
			},
		},
		deployed: map[string]string{},
	}
}

func (f *Fake) Status(context.Context) Status {
	return Status{Available: true, Python: "demo", Source: "demo", Reason: ""}
}

func (f *Fake) Games(context.Context) ([]Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Game, 0, len(f.games))
	for _, game := range f.games {
		if profile, ok := f.deployed[game.ID]; ok {
			game.DeployActive = true
			game.DeployedProfile = profile
		}
		out = append(out, game)
	}
	return out, nil
}

func (f *Fake) Setup(context.Context, Sink) error { return nil }

func (f *Fake) Deploy(ctx context.Context, game, profile string, sink Sink) error {
	if profile == "" {
		profile = "default"
	}
	if err := f.work(ctx, sink, "deploying", 6); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.find(game) == nil {
		return &Error{Kind: KindNotFound, Message: fmt.Sprintf("Amethyst has no handler for '%s'", game)}
	}
	f.deployed[game] = profile
	sink.log(fmt.Sprintf("Deploy complete: %s / %s", game, profile))
	return nil
}

func (f *Fake) Restore(ctx context.Context, game string, sink Sink) error {
	if err := f.work(ctx, sink, "restoring", 4); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.deployed, game)
	sink.log("Restore complete: " + game)
	return nil
}

func (f *Fake) Install(ctx context.Context, req InstallRequest, sink Sink) (string, error) {
	archive := req.Archive
	if req.URL != "" {
		archive = req.URL
		if err := f.work(ctx, sink, "downloading", 4); err != nil {
			return "", err
		}
	}
	if err := f.work(ctx, sink, "installing", 5); err != nil {
		return "", err
	}
	name := req.Name
	if name == "" {
		name = strings.TrimSuffix(baseName(archive), ".zip")
		name = strings.TrimSuffix(name, ".7z")
	}
	sink.log("Installed '" + name + "'.")
	return name, nil
}

func (f *Fake) Nexus(context.Context, string, string) (bool, error) { return false, nil }

func (f *Fake) find(id string) *Game {
	for index := range f.games {
		if f.games[index].ID == id {
			return &f.games[index]
		}
	}
	return nil
}

func (f *Fake) work(ctx context.Context, sink Sink, message string, steps int) error {
	for step := 1; step <= steps; step++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(120 * time.Millisecond):
		}
		sink.report(Progress{Current: step, Total: steps, Message: message})
	}
	return nil
}

func baseName(path string) string {
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		return path[index+1:]
	}
	return path
}
