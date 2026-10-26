package opti

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type LocalManager struct {
	storeDir string
	gh       *GitHubClient

	mu   sync.Mutex
	gpus []GPU

	extract func(archive, dest string) error
}

func NewLocalManager(storeDir string, gh *GitHubClient) *LocalManager {
	if gh == nil {
		gh = NewGitHubClient()
	}
	return &LocalManager{
		storeDir: storeDir,
		gh:       gh,
		extract: func(archive, dest string) error {
			return Extract7z(context.Background(), archive, dest)
		},
	}
}

func (m *LocalManager) versionPath(tag string) string {
	return filepath.Join(m.storeDir, tag)
}

func (m *LocalManager) DetectGPUs(ctx context.Context) ([]GPU, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gpus == nil {
		m.gpus = DetectGPUs()
		if m.gpus == nil {
			return nil, fmt.Errorf("no GPU detected (neither lspci nor sysfs reported one)")
		}
	}
	return m.gpus, nil
}

func (m *LocalManager) ListReleases(ctx context.Context) ([]Release, error) {
	return m.gh.ListReleases(ctx)
}

func (m *LocalManager) ListVersions(ctx context.Context) ([]string, error) {
	return LocalVersions(m.storeDir), nil
}

func (m *LocalManager) primaryGPU(ctx context.Context) (GPU, error) {
	gpus, err := m.DetectGPUs(ctx)
	if err != nil {
		return GPU{}, err
	}
	for _, g := range gpus {
		if g.Primary {
			return g, nil
		}
	}
	return gpus[0], nil
}

func (m *LocalManager) Download(ctx context.Context, tag string, progress func(Progress)) error {
	if tag == "" {
		return fmt.Errorf("a version tag is required")
	}
	if _, err := os.Stat(filepath.Join(m.versionPath(tag), "OptiScaler.dll")); err == nil {
		if progress != nil {
			done := 100.0
			progress(Progress{Phase: "downloading", Percent: &done, Done: true})
		}
		return nil
	}
	if err := os.MkdirAll(m.storeDir, 0o755); err != nil {
		return err
	}
	archive, err := m.gh.DownloadAsset(ctx, tag, m.storeDir, progress)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	dest := m.versionPath(tag)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := m.extract(archive, dest); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dest, "OptiScaler.dll")); err != nil {
		return fmt.Errorf("release %s extracted without OptiScaler.dll; not an OptiScaler archive?", tag)
	}
	if progress != nil {
		done := 100.0
		progress(Progress{Phase: "downloading", Percent: &done, Done: true})
	}
	return nil
}

func (m *LocalManager) Install(ctx context.Context, opts InstallOptions, progress func(Progress)) error {
	if opts.Version == "" {
		return fmt.Errorf("a version tag is required")
	}
	injection := HookName(opts.Injection)
	gameDir := GameDirFor(opts.InstallDir, opts.ExePath)
	if gameDir == "" {
		return fmt.Errorf("the game's install directory is unknown")
	}
	if _, err := os.Stat(filepath.Join(gameDir, "OptiScaler.dll")); err == nil {
		return fmt.Errorf("%s already contains an extracted OptiScaler.dll", gameDir)
	}

	if progress != nil {
		pct := 5.0
		progress(Progress{Phase: "downloading", Percent: &pct})
	}
	if err := m.Download(ctx, opts.Version, progress); err != nil {
		return err
	}
	if progress != nil {
		pct := 60.0
		progress(Progress{Phase: "installing", Percent: &pct})
	}
	if err := CopyVersionTree(m.versionPath(opts.Version), gameDir, injection); err != nil {
		return err
	}
	if progress != nil {
		pct := 85.0
		progress(Progress{Phase: "configuring", Percent: &pct})
	}
	gpu, err := m.primaryGPU(ctx)
	if err != nil {
		gpu = GPU{}
	}
	if err := ConfigureINI(gameDir, opts, gpu); err != nil {
		return err
	}
	if progress != nil {
		done := 100.0
		progress(Progress{Phase: "configuring", Percent: &done, Done: true})
	}
	return nil
}

func (m *LocalManager) Uninstall(ctx context.Context, gameID, exePath, installDir string) error {
	gameDir := GameDirFor(installDir, exePath)
	if gameDir == "" {
		return fmt.Errorf("the game's install directory is unknown")
	}
	return RemoveFromGameDir(gameDir)
}

func (m *LocalManager) Status(ctx context.Context, gameID, exePath, installDir string) ([]Status, error) {
	gameDir := GameDirFor(installDir, exePath)
	if gameDir == "" {
		return nil, nil
	}
	manifest, ok := ReadManifest(gameDir)
	if !ok {
		return nil, nil
	}
	return []Status{{
		GameID:       gameID,
		Installed:    true,
		Version:      manifest.Version,
		InjectionDLL: manifest.InjectionDLL,
		GameDir:      gameDir,
	}}, nil
}

func DefaultSpoof(gpu GPU) bool { return defaultSpoof(gpu) }
