package opti

import "context"

type GPU struct {
	Name         string `json:"name"`
	Vendor       string `json:"vendor"`
	Architecture string `json:"architecture"`
	Primary      bool   `json:"primary"`
}

type Release struct {
	Tag  string `json:"tag"`
	URL  string `json:"url"`
	Date string `json:"date"`
}

type Status struct {
	GameID       string `json:"game_id"`
	Installed    bool   `json:"installed"`
	Version      string `json:"version,omitempty"`
	InjectionDLL string `json:"injection_dll,omitempty"`
	Int8         bool   `json:"int8,omitempty"`
	GameDir      string `json:"game_dir,omitempty"`

	LaunchOptions string `json:"launch_options,omitempty"`
}

type InstallOptions struct {
	GameID     string
	ExePath    string
	InstallDir string
	Version    string
	Injection  string
	Upscaler   string
	FrameGen   bool
	FGInput    string
	FGOutput   string
	SpoofDLSS  bool
	Int8       bool
}

type Progress struct {
	Phase   string   `json:"phase"`
	Percent *float64 `json:"percent"`
	Done    bool     `json:"done"`
}

type Manager interface {
	DetectGPUs(ctx context.Context) ([]GPU, error)
	ListReleases(ctx context.Context) ([]Release, error)
	ListVersions(ctx context.Context) ([]string, error)
	Download(ctx context.Context, tag string, progress func(Progress)) error
	Install(ctx context.Context, opts InstallOptions, progress func(Progress)) error
	Uninstall(ctx context.Context, gameID, exePath, installDir string) error
	Status(ctx context.Context, gameID, exePath, installDir string) ([]Status, error)
}
