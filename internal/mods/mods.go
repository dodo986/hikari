package mods

import (
	"context"
	"errors"
)

// Kinds carry the backend's own classification of a failure so the daemon can
// answer with the matching JSON-RPC code.
const (
	KindConfig       = "config"
	KindUnavailable  = "unavailable"
	KindNotFound     = "notfound"
	KindUnconfigured = "unconfigured"
	KindUnsupported  = "unsupported"
	KindUsage        = "usage"
	KindFailure      = "failure"
)

type Error struct {
	Kind    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func KindOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ""
}

// Game is one Amethyst game handler. ID is Amethyst's game_id and is the
// handle every other call takes.
type Game struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	SteamAppID      string   `json:"steam_app_id,omitempty"`
	Configured      bool     `json:"configured"`
	Path            string   `json:"path,omitempty"`
	Profiles        []string `json:"profiles"`
	DeployedProfile string   `json:"deployed_profile,omitempty"`
	DeployActive    bool     `json:"deploy_active"`
}

type Status struct {
	Available bool   `json:"available"`
	Python    string `json:"python,omitempty"`
	Source    string `json:"src,omitempty"`
	Reason    string `json:"reason"`
}

type Progress struct {
	Current int
	Total   int
	Message string
}

type Sink struct {
	Log      func(line string)
	Progress func(p Progress)
}

func (s Sink) log(line string) {
	if s.Log != nil {
		s.Log(line)
	}
}

func (s Sink) report(p Progress) {
	if s.Progress != nil {
		s.Progress(p)
	}
}

type InstallRequest struct {
	Game    string
	Profile string
	Archive string
	URL     string
	Name    string
}

type Service interface {
	Status(ctx context.Context) Status
	Games(ctx context.Context) ([]Game, error)
	Setup(ctx context.Context, sink Sink) error
	Deploy(ctx context.Context, game, profile string, sink Sink) error
	Restore(ctx context.Context, game string, sink Sink) error
	Install(ctx context.Context, req InstallRequest, sink Sink) (string, error)
	Nexus(ctx context.Context, action, key string) (bool, error)
}
