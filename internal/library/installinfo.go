package library

import "context"

type InstallInfo struct {
	Title        string `json:"title,omitempty"`
	Version      string `json:"version,omitempty"`
	DownloadSize int64  `json:"download_size"`
	DiskSize     int64  `json:"disk_size"`
	DLCs         []DLC  `json:"dlcs,omitempty"`
}

type DLC struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	DownloadSize int64  `json:"download_size,omitempty"`
}

type InstallInfoProvider interface {
	InstallInfo(ctx context.Context, id, platform string) (InstallInfo, error)
}

type Updater interface {
	Update(ctx context.Context, id string, opts InstallOptions, progress func(InstallProgress)) error
}
