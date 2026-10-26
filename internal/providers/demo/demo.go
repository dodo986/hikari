package demo

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"hikari/internal/library"
)

type Seeder interface {
	Seed(url string, r io.Reader) error
}

type game struct {
	library.Game
	installing bool
}

type Provider struct {
	store string
	state *state
}

type state struct {
	mu       sync.Mutex
	games    map[string]*game
	seeded   bool
	authUser map[string]bool
}

func New(log *slog.Logger) map[string]*Provider {
	s := &state{games: map[string]*game{}, authUser: map[string]bool{}}
	for _, spec := range sampleGames() {
		g := spec
		s.games[g.Key()] = &game{Game: g}
	}
	s.authUser["epic"] = true
	s.authUser["gog"] = true
	s.authUser["amazon"] = true

	out := map[string]*Provider{}
	for _, store := range []string{
		library.StoreEpic, library.StoreGOG, library.StoreAmazon, library.StoreSteam,
	} {
		out[store] = &Provider{store: store, state: s}
	}
	return out
}

func (p *Provider) SeedArtwork(seeder Seeder) error {
	p.state.mu.Lock()
	already := p.state.seeded
	var urls []string
	for _, g := range p.state.games {
		if g.Artwork.Cover != "" {
			urls = append(urls, g.Artwork.Cover)
		}
		if g.Artwork.Square != "" {
			urls = append(urls, g.Artwork.Square)
		}
	}
	p.state.seeded = true
	p.state.mu.Unlock()
	if already {
		return nil
	}
	for i, url := range urls {
		r, w := io.Pipe()
		go func(i int) {
			writePlaceholder(w, i)
			w.Close()
		}(i)
		if err := seeder.Seed(url, r); err != nil {
			return err
		}
	}
	return nil
}

func writePlaceholder(w io.Writer, seed int) {
	const wpx, hpx = 480, 640
	img := image.NewRGBA(image.Rect(0, 0, wpx, hpx))
	hue := rand.New(rand.NewSource(int64(seed))).Float64()
	c1 := hsl(hue, 0.45, 0.28)
	c2 := hsl(fmod(hue+0.12, 1), 0.5, 0.42)
	for y := 0; y < hpx; y++ {
		for x := 0; x < wpx; x++ {
			diag := (x + y*3/4) % (wpx + hpx*3/4)
			t := float64(diag) / float64(wpx+hpx*3/4)
			img.Set(x, y, mix(c1, c2, t))
		}
	}
	_ = png.Encode(w, img)
}

func hsl(h, s, l float64) color.RGBA {
	r, g, b := hslToRGB(fmod(h, 1), s, l)
	return color.RGBA{r, g, b, 255}
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	var r, g, b float64
	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}
	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}

func hueToRGB(p, q, t float64) float64 {
	t = fmod(t, 1)
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		255,
	}
}

func fmod(f, m float64) float64 {
	for f < 0 {
		f += m
	}
	for f >= m {
		f -= m
	}
	return f
}

func (p *Provider) Store() string { return p.store }

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	var out []library.Game
	for _, g := range p.state.games {
		if g.Store == p.store {
			out = append(out, g.Game)
		}
	}
	return out, nil
}

func (p *Provider) Install(ctx context.Context, id string, opts library.InstallOptions, progress func(library.InstallProgress)) error {
	p.state.mu.Lock()
	g, ok := p.state.games[p.store+"/"+id]
	if ok {
		g.installing = true
	}
	p.state.mu.Unlock()
	if !ok {
		return fmt.Errorf("demo: no game %q in store %q", id, p.store)
	}
	if progress != nil {
		progress(library.InstallProgress{State: "downloading"})
	}
	for pct := 5.0; pct <= 100; pct += 5 {
		select {
		case <-ctx.Done():
			if progress != nil {
				progress(library.InstallProgress{State: "canceled"})
			}
			p.state.mu.Lock()
			g.installing = false
			p.state.mu.Unlock()
			return library.ErrInstallCanceled
		case <-time.After(25 * time.Millisecond):
		}
		if progress != nil {
			eta := fmt.Sprintf("00:%02d:%02d", int((100-pct)/5), 0)
			speed := 4.0 + 2*pct/100
			progress(library.InstallProgress{
				State: "downloading", Percent: &pct,
				Bytes: fmt.Sprintf("%.1fMB", pct*12.8), ETA: eta,
				DownSpeed: &speed, DiskSpeed: &speed,
			})
		}
	}
	p.state.mu.Lock()
	g.installing = false
	g.Installed = true
	g.InstallPath = opts.Path
	p.state.mu.Unlock()
	if progress != nil {
		done := 100.0
		progress(library.InstallProgress{State: "done", Percent: &done})
	}
	return nil
}

func (p *Provider) Uninstall(ctx context.Context, id string, keepFiles bool) error {
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	g, ok := p.state.games[p.store+"/"+id]
	if !ok || !g.Installed {
		return fmt.Errorf("demo: %q is not installed", id)
	}
	g.Installed = false
	g.InstallPath = ""
	return nil
}

func (p *Provider) InstallInfo(ctx context.Context, id, platform string) (library.InstallInfo, error) {
	p.state.mu.Lock()
	g, ok := p.state.games[p.store+"/"+id]
	p.state.mu.Unlock()
	if !ok {
		return library.InstallInfo{}, fmt.Errorf("demo: no game %q", id)
	}
	info := library.InstallInfo{
		Title:        g.Title,
		Version:      g.Version,
		DownloadSize: int64(len(g.Title))*1_000_000 + 500_000_000,
		DiskSize:     int64(len(g.Title))*2_000_000 + 1_200_000_000,
	}
	if strings.Contains(g.Title, "Collection") || g.Store == library.StoreGOG {
		info.DLCs = []library.DLC{{ID: id + "-dlc1", Title: g.Title + " Soundtrack", DownloadSize: 250_000_000}}
	}
	return info, nil
}

func (p *Provider) AuthStatus(ctx context.Context) (library.AuthStatus, error) {
	p.state.mu.Lock()
	logged := p.state.authUser[p.store]
	p.state.mu.Unlock()
	st := library.AuthStatus{LoggedIn: logged}
	if logged {
		st.User = "DemoPlayer"
		st.UserID = "demo-0001"
	}
	return st, nil
}

func (p *Provider) BeginLogin(ctx context.Context) (library.LoginInfo, error) {
	return library.LoginInfo{
		URL:  "https://demo.hikari.invalid/login",
		Hint: "demo mode: any code completes the login",
	}, nil
}

func (p *Provider) CompleteLogin(ctx context.Context, code string) (library.AuthStatus, error) {
	if strings.TrimSpace(code) == "" {
		return library.AuthStatus{}, fmt.Errorf("demo: a code is still required")
	}
	p.state.mu.Lock()
	p.state.authUser[p.store] = true
	p.state.mu.Unlock()
	return p.AuthStatus(ctx)
}

func (p *Provider) Logout(ctx context.Context) error {
	p.state.mu.Lock()
	p.state.authUser[p.store] = false
	p.state.mu.Unlock()
	return nil
}

func (p *Provider) Refresh(ctx context.Context) (int, error) {
	games, _ := p.List(ctx)
	return len(games), nil
}

func (p *Provider) DemoLaunchLife(g library.Game) time.Duration {
	return 90 * time.Second
}

func sampleGames() []library.Game {
	mk := func(store, id, title, runner, platform string, installed bool, size string) library.Game {
		return library.Game{
			Store: store, ID: id, Title: title, Runner: runner,
			Source: library.SourceDemo, Platform: platform, Installed: installed,
			InstallSize: size, CanRunOffline: true,
			Version: "1.0", Developer: "Demo Studio",
			Artwork: library.Artwork{
				Cover:  "https://demo.hikari.invalid/art/" + id + "-cover.jpg",
				Square: "https://demo.hikari.invalid/art/" + id + "-square.jpg",
			},
		}
	}
	epic := library.StoreEpic
	gog := library.StoreGOG
	amazon := library.StoreAmazon
	steam := library.StoreSteam
	games := []library.Game{
		mk(epic, "demo-epic-1", "Sushi Racer Ultra", library.RunnerLegendary, "Windows", true, "24 GiB"),
		mk(epic, "demo-epic-2", "Nori Puzzles", library.RunnerLegendary, "Windows", true, "800 MiB"),
		mk(epic, "demo-epic-3", "Mochi Mansion", library.RunnerLegendary, "Windows", false, "48 GiB"),
		mk(epic, "demo-epic-4", "Tanuki Trails", library.RunnerLegendary, "Windows", false, "12 GiB"),
		mk(gog, "demo-gog-1", "Wasabi Wars II", library.RunnerGogdl, "windows", true, "31 GiB"),
		mk(gog, "demo-gog-2", "Onigiri Collection", library.RunnerGogdl, "windows", true, "6 GiB"),
		mk(gog, "demo-gog-3", "Dango Derby", library.RunnerGogdl, "Linux", false, "3 GiB"),
		mk(amazon, "demo-amzn-1", "Tamago Quest", library.RunnerNile, "Windows", true, "18 GiB"),
		mk(amazon, "demo-amzn-2", "Bento Battlegrounds", library.RunnerNile, "Windows", false, "29 GiB"),
		mk(steam, "demo-steam-1", "Ramen Rally", library.RunnerSteam, "Linux", true, "9 GiB"),
		mk(steam, "demo-steam-2", "Udon Underground", library.RunnerSteam, "Linux", true, "15 GiB"),
		mk(steam, "demo-steam-3", "Katsu Kart", library.RunnerSteam, "Linux", false, "22 GiB"),
	}
	for i := range games {
		if games[i].Installed {
			games[i].InstallPath = "/opt/demo-games/" + games[i].ID
		}
		games[i].Executable = games[i].ID + "/Binaries/Win64/Game.exe"
	}
	return games
}
