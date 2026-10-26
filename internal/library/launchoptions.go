package library

import "context"

type LaunchOption struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Params []string `json:"params,omitempty"`
	Exe    string   `json:"exe,omitempty"`
	DLCApp string   `json:"dlc_app,omitempty"`
}

type LaunchOptionProvider interface {
	LaunchOptions(ctx context.Context, g Game) ([]LaunchOption, error)
}

type Repairer interface {
	Repair(ctx context.Context, id string, opts InstallOptions, progress func(InstallProgress)) error
}

type SaveOptions struct {
	Direction string
	SavePath  string
}

type SaveSyncer interface {
	SyncSaves(ctx context.Context, id string, opts SaveOptions) (string, error)
}
