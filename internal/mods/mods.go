package mods

import (
	"context"
	"errors"
	"time"
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
	NexusDomain     string   `json:"nexus_domain,omitempty"`
}

// Mod is one Nexus search hit, as much of it as a list needs.
type Mod struct {
	ModID        int    `json:"mod_id"`
	Name         string `json:"name"`
	Summary      string `json:"summary,omitempty"`
	Author       string `json:"author,omitempty"`
	Version      string `json:"version,omitempty"`
	Downloads    int    `json:"downloads"`
	Endorsements int    `json:"endorsements"`
	Adult        bool   `json:"adult,omitempty"`
	Created      string `json:"created,omitempty"`
	Updated      string `json:"updated,omitempty"`
	Category     string `json:"category,omitempty"`
	PictureURL   string `json:"picture_url,omitempty"`
}

// ModFile is one downloadable file of a mod.
type ModFile struct {
	FileID   int    `json:"file_id"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Category string `json:"category,omitempty"`
	Size     string `json:"size,omitempty"`
	Uploaded string `json:"uploaded,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
}

// Browse is one page of Nexus search results.
type Browse struct {
	Game    string `json:"game"`
	Domain  string `json:"domain,omitempty"`
	Page    int    `json:"page"`
	More    bool   `json:"more"`
	Results []Mod  `json:"results"`
}

// Files is one mod's file listing, newest upload first.
type Files struct {
	Mod   Mod       `json:"mod"`
	Files []ModFile `json:"files"`
}

// InstalledMod is one entry of a profile's modlist, highest priority first.
type InstalledMod struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// Installed is a profile's modlist.
type Installed struct {
	Game    string         `json:"game"`
	Profile string         `json:"profile"`
	Mods    []InstalledMod `json:"mods"`
}

// Account is who the stored Nexus key belongs to.
type Account struct {
	Stored  bool   `json:"stored"`
	Mode    string `json:"mode,omitempty"` // "oauth" after a browser login, "key" for a pasted key
	User    string `json:"user,omitempty"`
	Premium bool   `json:"premium"`
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
	// AuthURL carries the page the user has to authorise on.
	AuthURL func(url string)
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

func (s Sink) authURL(url string) {
	if s.AuthURL != nil {
		s.AuthURL(url)
	}
}

// Finding is one thing the mod health check saw; Fixable means a redeploy heals it.
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Mod      string `json:"mod,omitempty"`
	File     string `json:"file,omitempty"`
	Detail   string `json:"detail"`
	Fixable  bool   `json:"fixable"`
}

// Broken counts the findings that mean no mod can be loading.
func Broken(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == "error" {
			n++
		}
	}
	return n
}

// Fixable reports whether a redeploy would clear any finding.
func Fixable(findings []Finding) bool {
	for _, f := range findings {
		if f.Fixable {
			return true
		}
	}
	return false
}

// Requirement is one Nexus-listed mod another mod needs.
type Requirement struct {
	ModID    int    `json:"mod_id"`
	Name     string `json:"name"`
	External bool   `json:"external,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// Missing is an installed mod with requirements the profile does not satisfy.
type Missing struct {
	Mod   string        `json:"mod"`
	ModID int           `json:"mod_id"`
	Needs []Requirement `json:"needs"`
}

type InstallRequest struct {
	Game    string
	Profile string
	Archive string
	URL     string
	Name    string
	ModID   int
	FileID  int
}

type Service interface {
	Status(ctx context.Context) Status
	Games(ctx context.Context) ([]Game, error)
	Setup(ctx context.Context, sink Sink) error
	Deploy(ctx context.Context, game, profile string, sink Sink) error
	Restore(ctx context.Context, game string, sink Sink) error
	Install(ctx context.Context, req InstallRequest, sink Sink) (string, error)
	Nexus(ctx context.Context, action, key string) (bool, error)
	Account(ctx context.Context) (Account, error)
	Login(ctx context.Context, sink Sink) (Account, error)
	Logout(ctx context.Context) error
	Browse(ctx context.Context, game, query, sort string, page int) (Browse, error)
	Files(ctx context.Context, game string, modID int) (Mod, []ModFile, error)
	Downloads(ctx context.Context, game string, modID, fileID int) (string, error)
	Toggle(ctx context.Context, game, profile, name string) (bool, error)
	Remove(ctx context.Context, game, profile, name string, sink Sink) error
	Missing(ctx context.Context, game, profile string) ([]Missing, error)
	Overrides(ctx context.Context, game string) (map[string]string, error)
	Health(ctx context.Context, game, profile string, network bool) ([]Finding, error)
	StopNote(ctx context.Context, root string, from, to time.Time) (*StopNote, error)
	Installed(ctx context.Context, game, profile string) (Installed, error)
}
