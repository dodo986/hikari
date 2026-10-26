package runtimes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"hikari/internal/archive"
)

type Name string

const (
	EAC      Name = "eac_runtime"
	BattlEye Name = "battleye_runtime"
)

type Installer struct {
	Dir     string
	API     string
	Client  *http.Client
	Extract func(ctx context.Context, archive, dest string) error
	Log     *slog.Logger
}

func NewInstaller(dir string, log *slog.Logger) *Installer {
	if log == nil {
		log = slog.Default()
	}
	return &Installer{
		Dir:     dir,
		API:     "https://lutris.net/api/runtimes",
		Client:  &http.Client{Timeout: 60 * time.Second},
		Extract: archive.Extract,
		Log:     log,
	}
}

func (i *Installer) Path(name Name) string {
	return filepath.Join(i.Dir, string(name))
}

func (i *Installer) Installed(name Name) (string, bool) {
	dir := i.Path(name)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "", false
	}
	return dir, true
}

func (i *Installer) Ensure(ctx context.Context, name Name) (string, error) {
	if dir, ok := i.Installed(name); ok {
		return dir, nil
	}
	url, version, err := i.lookup(ctx, name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(i.Dir, 0o755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(i.Dir, string(name)+".part")
	if err := i.download(ctx, url, archivePath); err != nil {
		return "", err
	}
	defer os.Remove(archivePath)

	tmp := i.Path(name) + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	i.Log.Info("extracting runtime", "runtime", name, "version", version)
	if err := i.Extract(ctx, archivePath, tmp); err != nil {
		return "", fmt.Errorf("extract %s: %w", name, err)
	}
	if err := flatten(tmp); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, "current_version"), []byte(version), 0o644); err != nil {
		return "", err
	}
	_ = os.RemoveAll(i.Path(name))
	if err := os.Rename(tmp, i.Path(name)); err != nil {
		return "", err
	}
	return i.Path(name), nil
}

func (i *Installer) lookup(ctx context.Context, name Name) (url, version string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.API, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := i.Client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("runtime list: status %d", resp.StatusCode)
	}
	var list []struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", "", err
	}
	for _, r := range list {
		if r.Name == string(name) && r.URL != "" {
			return r.URL, r.CreatedAt, nil
		}
	}
	return "", "", fmt.Errorf("runtime %s not in the Lutris list", name)
}

func (i *Installer) download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := i.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func flatten(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var only string
	for _, e := range entries {
		if !e.IsDir() {
			return nil
		}
		if only != "" {
			return nil
		}
		only = e.Name()
	}
	if only == "" {
		return nil
	}
	inner := filepath.Join(dir, only)
	kids, err := os.ReadDir(inner)
	if err != nil {
		return err
	}
	for _, k := range kids {
		if err := os.Rename(filepath.Join(inner, k.Name()), filepath.Join(dir, k.Name())); err != nil {
			return err
		}
	}
	return os.Remove(inner)
}
