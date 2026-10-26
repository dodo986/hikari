package dxvk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hikari/internal/archive"
)

type Tool string

const (
	DXVK  Tool = "dxvk"
	VKD3D Tool = "vkd3d"
	NVAPI Tool = "dxvk-nvapi"
)

type spec struct {
	repo    string
	archive string
	suffix  string
	x86Dir  string
}

var specs = map[Tool]spec{
	DXVK:  {repo: "doitsujin/dxvk", archive: "dxvk-", suffix: ".tar.gz", x86Dir: "x32"},
	VKD3D: {repo: "HansKristian-Work/vkd3d-proton", archive: "vkd3d-proton-", suffix: ".tar.zst", x86Dir: "x86"},
	NVAPI: {repo: "jp7677/dxvk-nvapi", archive: "dxvk-nvapi-", suffix: ".tar.gz", x86Dir: "x86"},
}

func ToolFromString(s string) (Tool, bool) {
	t := Tool(s)
	_, ok := specs[t]
	return t, ok
}

type RegFunc func(ctx context.Context, dll string) error

type Installer struct {
	CacheDir string
	APIBase  string
	Client   *http.Client
	Log      *slog.Logger
	Extract  func(ctx context.Context, archive, dest string) error
}

func NewInstaller(cacheDir string, log *slog.Logger) *Installer {
	if log == nil {
		log = slog.Default()
	}
	return &Installer{
		CacheDir: cacheDir,
		APIBase:  "https://api.github.com",
		Client:   &http.Client{Timeout: 60 * time.Second},
		Log:      log,
		Extract:  archive.Extract,
	}
}

func (i *Installer) cachePath(tool Tool, version string) string {
	return filepath.Join(i.CacheDir, string(tool), version)
}

func (i *Installer) CheckDue(tool Tool, maxAge time.Duration) bool {
	st, err := os.Stat(i.stampPath(tool))
	return err != nil || time.Since(st.ModTime()) > maxAge
}

func (i *Installer) stampPath(tool Tool) string {
	return filepath.Join(i.CacheDir, string(tool), ".checked")
}

func (i *Installer) markChecked(tool Tool) {
	p := i.stampPath(tool)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, nil, 0o644)
}

func (i *Installer) Cached(tool Tool, version string) (string, bool) {
	if version == "" {
		return "", false
	}
	dir := i.cachePath(tool, version)
	if _, _, ok := Locate(tool, dir); !ok {
		return "", false
	}
	return dir, true
}

func (i *Installer) Ensure(ctx context.Context, tool Tool) (string, string, error) {
	sp, ok := specs[tool]
	if !ok {
		return "", "", fmt.Errorf("unknown tool %q", tool)
	}
	version, assetURL, size, err := i.latest(ctx, sp)
	if err != nil {
		return "", "", err
	}
	i.markChecked(tool)
	dir := i.cachePath(tool, version)
	if _, _, ok := Locate(tool, dir); ok {
		return version, dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	archive := dir + ".part"
	if err := i.download(ctx, assetURL, archive, size); err != nil {
		return "", "", err
	}
	defer os.Remove(archive)
	i.Log.Info("extracting tool", "tool", tool, "version", version)
	if err := i.Extract(ctx, archive, dir); err != nil {
		return "", "", fmt.Errorf("extract %s %s: %w", tool, version, err)
	}
	if _, _, ok := Locate(tool, dir); !ok {
		return "", "", fmt.Errorf("%s %s extracted but no x64 directory found", tool, version)
	}
	return version, dir, nil
}

func (i *Installer) latest(ctx context.Context, sp spec) (version, url string, size int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.APIBase+"/repos/"+sp.repo+"/releases/latest", nil)
	if err != nil {
		return "", "", 0, err
	}
	resp, err := i.Client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, fmt.Errorf("%s releases: status %d", sp.repo, resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", 0, err
	}
	want := map[string]bool{
		sp.archive + strings.TrimPrefix(rel.TagName, "v") + sp.suffix: true,
		sp.archive + rel.TagName + sp.suffix:                          true,
	}
	for _, a := range rel.Assets {
		if want[a.Name] {
			return rel.TagName, a.URL, a.Size, nil
		}
	}
	return "", "", 0, fmt.Errorf("%s %s: no %s*%s asset in release", sp.repo, rel.TagName, sp.archive, sp.suffix)
}

func (i *Installer) download(ctx context.Context, url, dest string, size int64) error {
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
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if size > 0 && n != size {
		return fmt.Errorf("download %s: got %d bytes, release says %d", url, n, size)
	}
	return nil
}
