package steam

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hikari/internal/library"
)

type Provider struct {
	roots []string
	home  string
}

func New() *Provider { return &Provider{} }

func NewWithRoots(roots ...string) *Provider { return &Provider{roots: roots} }

func (p *Provider) Store() string { return library.StoreSteam }

func (p *Provider) steamRoots() []string {
	if p.roots != nil {
		return p.roots
	}
	home := p.home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
	}
}

func libraryFolders(root string) []string {
	vdfPath := filepath.Join(root, "steamapps", "libraryfolders.vdf")
	b, err := os.ReadFile(vdfPath)
	if err != nil {
		return nil
	}
	doc, err := parseVDF(string(b))
	if err != nil {
		return nil
	}
	var out []string
	for _, folder := range vdfMaps(doc, "libraryfolders") {
		path := vdfString(folder, "path")
		if path == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(path, "steamapps")); err == nil && st.IsDir() {
			out = append(out, path)
		}
	}
	return out
}

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	var games []library.Game
	for _, root := range p.steamRoots() {
		for _, lib := range libraryFolders(root) {
			games = append(games, gamesInLibrary(lib)...)
		}
	}
	sort.Slice(games, func(i, j int) bool { return games[i].ID < games[j].ID })
	return games, nil
}

func gamesInLibrary(lib string) []library.Game {
	entries, err := os.ReadDir(filepath.Join(lib, "steamapps"))
	if err != nil {
		return nil
	}
	var games []library.Game
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "appmanifest_") || !strings.HasSuffix(name, ".acf") {
			continue
		}
		g, ok := parseManifest(filepath.Join(lib, "steamapps", name), lib)
		if ok {
			games = append(games, g)
		}
	}
	return games
}

func parseManifest(path, lib string) (library.Game, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return library.Game{}, false
	}
	doc, err := parseVDF(string(b))
	if err != nil {
		return library.Game{}, false
	}
	appState, ok := doc["AppState"].(map[string]any)
	if !ok {
		return library.Game{}, false
	}
	id := vdfString(appState, "appid")
	name := vdfString(appState, "name")
	if id == "" || name == "" {
		return library.Game{}, false
	}
	installdir := vdfString(appState, "installdir")
	if installdir == "" {
		installdir = name
	}
	g := library.Game{
		Store:    library.StoreSteam,
		ID:       id,
		Title:    name,
		Runner:   library.RunnerSteam,
		Source:   library.SourceHikari,
		Platform: "Linux",
		Version:  vdfString(appState, "buildid"),
	}
	installPath := filepath.Join(lib, "steamapps", "common", installdir)
	g.InstallPath = installPath
	g.Installed = vdfString(appState, "StateFlags") == "4"
	if st, err := os.Stat(installPath); g.Installed && (err != nil || !st.IsDir()) {
		g.Installed = false
	}
	g.Artwork = steamArt(lib, id)
	return g, true
}

// steamArt prefers the images Steam itself uses at their double resolution and
// keeps the local cache as a fallback, since the cache is often the small copy.
func steamArt(lib, appID string) library.Artwork {
	dir := filepath.Join(lib, "appcache", "librarycache", appID)
	cover := cdn(appID, "library_600x900_2x.jpg", "library_600x900.jpg")
	if c := cacheImage(dir, "library_600x900_2x.jpg", "library_600x900.jpg"); c != "" {
		cover = c
	}
	square := cdn(appID, "library_hero.jpg", "library_header.jpg")
	if s := cacheImage(dir, "library_hero.jpg", "library_header.jpg", "header.jpg"); s != "" {
		square = s
	}
	return library.Artwork{
		Cover:  cover,
		Square: square,
		Logo:   cdn(appID, "logo.png"),
	}
}

func cdn(appID string, names ...string) string {
	if appID == "" {
		return ""
	}
	return fmt.Sprintf(steamArtURL, appID, names[0])
}

const steamArtURL = "https://cdn.akamai.steamstatic.com/steam/apps/%s/%s"

// SteamCDN exposes the artwork URLs for an app id so other stores can borrow
// them once a title is matched to Steam.
func SteamCDN(appID string) library.Artwork {
	if appID == "" {
		return library.Artwork{}
	}
	return library.Artwork{
		Cover:  fmt.Sprintf(steamArtURL, appID, "library_600x900_2x.jpg"),
		Square: fmt.Sprintf(steamArtURL, appID, "library_hero.jpg"),
		Logo:   fmt.Sprintf(steamArtURL, appID, "logo.png"),
	}
}

func cacheImage(dir string, names ...string) string {
	for _, name := range names {
		if p := filepath.Join(dir, name); fileExists(p) {
			return p
		}
	}
	for _, name := range names {
		matches, _ := filepath.Glob(filepath.Join(dir, "*", name))
		for _, p := range matches {
			if fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func LaunchOptions(hookDLL string) string {
	base := strings.TrimSuffix(hookDLL, ".dll")
	if base == "" {
		base = "dxgi"
	}
	return "WINEDLLOVERRIDES=" + base + "=n,b %command%"
}
