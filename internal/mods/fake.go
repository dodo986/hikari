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
	signedIn bool
}

func NewFake() *Fake {
	return &Fake{
		games: []Game{
			{
				ID: "skyrim_se", Name: "Skyrim Special Edition", SteamAppID: "489830",
				Configured: true, Path: "/games/Skyrim Special Edition",
				Profiles: []string{"default", "vanilla-plus"}, NexusDomain: "skyrimspecialedition",
			},
			{
				ID: "cyberpunk_2077", Name: "Cyberpunk 2077", SteamAppID: "1091500",
				Configured: true, Path: "/games/Cyberpunk 2077",
				Profiles: []string{"default"}, NexusDomain: "cyberpunk2077",
			},
			{
				ID: "baldurs_gate_3", Name: "Baldur's Gate 3", SteamAppID: "1086940",
				Configured: false, Profiles: []string{}, NexusDomain: "baldursgate3",
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
	if req.ModID > 0 {
		mod, _, err := f.Files(ctx, req.Game, req.ModID)
		if err != nil {
			return "", err
		}
		if err := f.work(ctx, sink, "downloading", 4); err != nil {
			return "", err
		}
		sink.log("Downloaded " + mod.Name)
		if req.Name == "" {
			req.Name = mod.Name
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

func (f *Fake) Nexus(context.Context, string, string) (bool, error) { return true, nil }

func (f *Fake) Account(context.Context) (Account, error) {
	if !f.signedIn {
		return Account{}, nil
	}
	return Account{Stored: true, Mode: "oauth", User: "demoplayer"}, nil
}

// Login stands in for the browser round trip so the page can be seen signed in.
func (f *Fake) Login(_ context.Context, sink Sink) (Account, error) {
	sink.log("waiting for you to authorise Hikari on the Nexus page")
	sink.authURL("https://users.nexusmods.com/oauth/authorize?client_id=amethyst&demo=1")
	f.mu.Lock()
	f.signedIn = true
	f.mu.Unlock()
	return Account{Stored: true, Mode: "oauth", User: "demoplayer"}, nil
}

func (f *Fake) Logout(context.Context) error {
	f.mu.Lock()
	f.signedIn = false
	f.mu.Unlock()
	return nil
}

// demoMods is the demo daemon's Nexus catalogue, so the page has something to list.
var demoMods = map[string][]Mod{
	"skyrimspecialedition": {
		{ModID: 12604, Name: "SkyUI", Author: "SkyUI Team", Version: "5.2", Downloads: 41_000_000, Endorsements: 480_000, Category: "User Interface", Created: "2024-02-01T00:00:00Z", Updated: "2026-03-11T18:30:00Z", Summary: "A user interface for Skyrim that is both more powerful and more console friendly."},
		{ModID: 266, Name: "Unofficial Skyrim Special Edition Patch", Author: "Unofficial Patch Project Team", Version: "4.3.9a", Downloads: 22_400_000, Endorsements: 320_000, Category: "Bugfixes", Created: "2024-02-01T00:00:00Z", Updated: "2026-10-19T07:15:00Z", Summary: "Comprehensive bugfixing for Skyrim Special Edition."},
		{ModID: 19518, Name: "Address Library for SKSE Plugins", Author: "meh321", Version: "12", Downloads: 18_900_000, Endorsements: 210_000, Category: "Utilities", Created: "2024-02-01T00:00:00Z", Updated: "2026-08-30T21:40:00Z", Summary: "Version independent addresses for SKSE plugins."},
		{ModID: 75978, Name: "Alternate Start - Live Another Life", Author: "Arthmoor", Version: "4.2.1", Downloads: 12_100_000, Endorsements: 130_000, Category: "Quests", Created: "2024-02-01T00:00:00Z", Updated: "2026-05-14T11:05:00Z", Summary: "Change the way you start a new game."},
		{ModID: 1137, Name: "Static Mesh Improvement Mod", Author: "Brumbek", Version: "2.08", Downloads: 9_600_000, Endorsements: 120_000, Category: "Models and Textures", Created: "2024-02-01T00:00:00Z", Updated: "2026-07-02T16:20:00Z", Summary: "Replaces 900+ meshes with improved models."},
		{ModID: 242, Name: "Total Character Makeover", Author: "ltho", Version: "1.2", Downloads: 4_300_000, Endorsements: 60_000, Category: "Models and Textures", Created: "2024-02-01T00:00:00Z", Updated: "2025-12-09T13:45:00Z", Summary: "High resolution face and body textures."},
	},
	"cyberpunk2077": {
		{ModID: 107, Name: "Cyber Engine Tweaks", Author: "Yamashi", Version: "1.34.0", Downloads: 6_800_000, Endorsements: 90_000, Category: "Gameplay", Created: "2024-02-01T00:00:00Z", Updated: "2026-10-20T10:00:00Z", Summary: "Scripting framework for Cyberpunk 2077, with a console and a Lua API."},
		{ModID: 3421, Name: "RED4ext", Author: "WopsS", Version: "1.25.0", Downloads: 5_400_000, Endorsements: 74_000, Category: "Utilities", Created: "2024-02-01T00:00:00Z", Updated: "2026-10-22T19:10:00Z", Summary: "A library that extends REDengine 4 by hooking into scripts."},
		{ModID: 7080, Name: "ArchiveXL", Author: "psiberx", Version: "1.19.0", Downloads: 4_100_000, Endorsements: 58_000, Category: "Utilities", Created: "2024-02-01T00:00:00Z", Updated: "2026-09-28T08:35:00Z", Summary: "Archive extension loader."},
	},
}

func (f *Fake) Browse(_ context.Context, game, query, sort string, page int) (Browse, error) {
	f.mu.Lock()
	known, signedIn := f.find(game), f.signedIn
	f.mu.Unlock()
	if !signedIn {
		return Browse{}, &Error{Kind: KindConfig, Message: "not signed in to Nexus; run `hikari mods login`"}
	}
	if known == nil {
		return Browse{}, &Error{Kind: KindNotFound, Message: fmt.Sprintf("Amethyst has no handler for '%s'", game)}
	}
	all := demoMods[known.NexusDomain]
	hits := make([]Mod, 0, len(all))
	for _, mod := range all {
		if query == "" || strings.Contains(strings.ToLower(mod.Name), strings.ToLower(query)) {
			hits = append(hits, mod)
		}
	}
	return Browse{Game: known.Name, Domain: known.NexusDomain, Page: page, More: false, Results: hits}, nil
}

func (f *Fake) Files(_ context.Context, game string, modID int) (Mod, []ModFile, error) {
	known := f.find(game)
	if known == nil {
		return Mod{}, nil, &Error{Kind: KindNotFound, Message: fmt.Sprintf("Amethyst has no handler for '%s'", game)}
	}
	for _, mod := range demoMods[known.NexusDomain] {
		if mod.ModID == modID {
			return mod, []ModFile{
				{FileID: 3, Name: mod.Name + " - optional files", Version: mod.Version, Category: "OPTIONAL", Size: "820 KB", Uploaded: "2026-10-24T09:00:00Z"},
				{FileID: 1, Name: mod.Name, Version: mod.Version, Category: "MAIN", Size: "12.4 MB", Uploaded: "2026-03-11T18:30:00Z", Primary: true},
				{FileID: 2, Name: mod.Name + " - 1.9 archive", Version: "1.9", Category: "OLD_VERSION", Size: "11.8 MB", Uploaded: "2025-06-02T12:00:00Z"},
			}, nil
		}
	}
	return Mod{}, nil, &Error{Kind: KindNotFound, Message: fmt.Sprintf("no Nexus mod %d for %s", modID, game)}
}

func (f *Fake) Installed(_ context.Context, game, profile string) (Installed, error) {
	known := f.find(game)
	if known == nil {
		return Installed{}, &Error{Kind: KindNotFound, Message: fmt.Sprintf("Amethyst has no handler for '%s'", game)}
	}
	if profile == "" {
		profile = "default"
	}
	out := Installed{Game: known.Name, Profile: profile}
	for _, mod := range demoMods[known.NexusDomain] {
		out.Mods = append(out.Mods, InstalledMod{Name: mod.Name, Enabled: true})
	}
	return out, nil
}

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
