package opti

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Fake struct {
	mu        sync.Mutex
	GPUs      []GPU
	Releases  []Release
	Versions  []string
	statuses  map[string]Status
	installs  []InstallOptions
	uninstall []string
	FailOn    string
}

func NewFake() *Fake {
	return &Fake{statuses: map[string]Status{}}
}

func (f *Fake) Installs() []InstallOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]InstallOptions{}, f.installs...)
}

func (f *Fake) Uninstalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.uninstall...)
}

func (f *Fake) DetectGPUs(ctx context.Context) ([]GPU, error) {
	if f.FailOn == "gpu" {
		return nil, fmt.Errorf("no GPU detected")
	}
	if len(f.GPUs) == 0 {
		return []GPU{{Name: "Fake GPU", Vendor: "AMD", Architecture: "rdna3", Primary: true}}, nil
	}
	return f.GPUs, nil
}

func (f *Fake) ListReleases(ctx context.Context) ([]Release, error) {
	if f.FailOn == "releases" {
		return nil, fmt.Errorf("network unavailable")
	}
	if len(f.Releases) == 0 {
		return []Release{{Tag: "v0.6.9", URL: "https://example.test/optiscaler/v0.6.9", Date: "2025-11-01"}}, nil
	}
	return f.Releases, nil
}

func (f *Fake) ListVersions(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string{}, f.Versions...)
	sort.Strings(out)
	return out, nil
}

func (f *Fake) Download(ctx context.Context, tag string, progress func(Progress)) error {
	if f.FailOn == "download" {
		return fmt.Errorf("download failed")
	}
	f.mu.Lock()
	found := false
	for _, v := range f.Versions {
		if v == tag {
			found = true
		}
	}
	if !found {
		f.Versions = append(f.Versions, tag)
	}
	f.mu.Unlock()
	if progress != nil {
		pct := 50.0
		progress(Progress{Phase: "downloading", Percent: &pct, Done: false})
		done := 100.0
		progress(Progress{Phase: "downloading", Percent: &done, Done: true})
	}
	return nil
}

func (f *Fake) Install(ctx context.Context, opts InstallOptions, progress func(Progress)) error {
	if f.FailOn == "install" {
		return fmt.Errorf("install failed")
	}
	if opts.ExePath == "" || opts.Version == "" {
		return fmt.Errorf("exe path and version are required")
	}
	if opts.Injection == "" {
		opts.Injection = "dxgi.dll"
	}
	f.mu.Lock()
	f.installs = append(f.installs, opts)
	f.statuses[opts.GameID] = Status{
		GameID: opts.GameID, Installed: true, Version: opts.Version,
		InjectionDLL: opts.Injection, Int8: opts.Int8,
		GameDir: dirOf(opts.ExePath),
	}
	f.mu.Unlock()
	if progress != nil {
		pct := 60.0
		progress(Progress{Phase: "installing", Percent: &pct})
		done := 100.0
		progress(Progress{Phase: "configuring", Percent: &done, Done: true})
	}
	return nil
}

func (f *Fake) Uninstall(ctx context.Context, gameID, exePath, installDir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.statuses[gameID]; !ok {
		return fmt.Errorf("no OptiScaler install found for %s", gameID)
	}
	delete(f.statuses, gameID)
	f.uninstall = append(f.uninstall, gameID)
	return nil
}

func (f *Fake) Status(ctx context.Context, gameID, exePath, installDir string) ([]Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if gameID != "" {
		if st, ok := f.statuses[gameID]; ok {
			return []Status{st}, nil
		}
		return nil, nil
	}
	out := make([]Status, 0, len(f.statuses))
	for _, st := range f.statuses {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GameID < out[j].GameID })
	return out, nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' && i > 0 {
			return path[:i]
		}
	}
	return "."
}
