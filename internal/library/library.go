package library

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"hikari/internal/config"
)

var ErrInstallCanceled = errors.New("install canceled")

const (
	StoreEpic   = "epic"
	StoreGOG    = "gog"
	StoreAmazon = "amazon"
	StoreSteam  = "steam"
	StoreHeroic = "heroic"
	StoreLocal  = "local"
)

const (
	RunnerLegendary = "legendary"
	RunnerGogdl     = "gogdl"
	RunnerNile      = "nile"
	RunnerSteam     = "steam"
	RunnerLocal     = "local"
	RunnerNone      = "none"
)

const (
	SourceHikari       = "hikari"
	SourceHeroicImport = "heroic-import"
	SourceDemo         = "demo"
)

type Artwork struct {
	Cover  string `json:"cover,omitempty"`
	Square string `json:"square,omitempty"`
	Logo   string `json:"logo,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

type Game struct {
	Store           string            `json:"store"`
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Developer       string            `json:"developer,omitempty"`
	Installed       bool              `json:"installed"`
	InstallPath     string            `json:"install_path,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	Version         string            `json:"version,omitempty"`
	Executable      string            `json:"executable,omitempty"`
	InstallSize     string            `json:"install_size,omitempty"`
	IsDLC           bool              `json:"is_dlc,omitempty"`
	Runner          string            `json:"runner"`
	Source          string            `json:"source"`
	Artwork         Artwork           `json:"artwork,omitempty"`
	ArtworkLocal    Artwork           `json:"artwork_local,omitempty"`
	CloudSaves      bool              `json:"cloud_saves,omitempty"`
	CanRunOffline   bool              `json:"can_run_offline,omitempty"`
	UpdateAvailable bool              `json:"update_available,omitempty"`
	Favorite        bool              `json:"favorite,omitempty"`
	LastPlayed      *time.Time        `json:"last_played,omitempty"`
	PlayCount       int               `json:"play_count,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func (g Game) Key() string { return g.Store + "/" + g.ID }

type Provider interface {
	Store() string
	List(ctx context.Context) ([]Game, error)
}

type SettingsSource interface {
	DefaultSettings(g Game) (settings config.GameSettings, found bool)
}

type Installer interface {
	Install(ctx context.Context, id string, opts InstallOptions, progress func(InstallProgress)) error
	Uninstall(ctx context.Context, id string, keepFiles bool) error
}

type InstallOptions struct {
	Path       string   `json:"path"`
	Platform   string   `json:"platform,omitempty"`
	DLCs       []string `json:"dlcs,omitempty"`
	Language   string   `json:"language,omitempty"`
	MaxWorkers int      `json:"max_workers,omitempty"`
}

type InstallProgress struct {
	State     string   `json:"state"`
	Percent   *float64 `json:"percent"`
	Bytes     string   `json:"bytes,omitempty"`
	ETA       string   `json:"eta,omitempty"`
	DownSpeed *float64 `json:"down_speed"`
	DiskSpeed *float64 `json:"disk_speed"`
}

type Service struct {
	providers []Provider
}

func NewService(providers ...Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) Provider(store string) Provider {
	for _, p := range s.providers {
		if p.Store() == store {
			return p
		}
	}
	return nil
}

func (s *Service) Providers() []Provider { return s.providers }

func (s *Service) List(ctx context.Context, store string, installedOnly bool) ([]Game, error) {
	games, firstErr := s.aggregate(ctx, store)
	if len(games) == 0 && firstErr != nil {
		return nil, firstErr
	}
	if installedOnly {
		kept := games[:0]
		for _, g := range games {
			if g.Installed {
				kept = append(kept, g)
			}
		}
		games = kept
	}
	sort.Slice(games, func(i, j int) bool {
		if games[i].Title != games[j].Title {
			return games[i].Title < games[j].Title
		}
		return games[i].ID < games[j].ID
	})
	return games, nil
}

func (s *Service) Get(ctx context.Context, store, id string) (Game, error) {
	games, err := s.aggregate(ctx, store)
	if err != nil && len(games) == 0 {
		return Game{}, err
	}
	for _, g := range games {
		if g.Store == store && g.ID == id {
			return g, nil
		}
	}
	return Game{}, fmt.Errorf("no game with id %q in store %q", id, store)
}

func (s *Service) aggregate(ctx context.Context, store string) ([]Game, error) {
	var firstErr error
	byKey := map[string]Game{}
	var order []string
	for _, p := range s.providers {
		list, err := p.List(ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, g := range list {
			if store != "" && g.Store != store {
				continue
			}
			key := g.Key()
			if _, seen := byKey[key]; !seen {
				order = append(order, key)
			}
			byKey[key] = g
		}
	}
	games := make([]Game, 0, len(order))
	for _, key := range order {
		games = append(games, byKey[key])
	}
	return games, firstErr
}
