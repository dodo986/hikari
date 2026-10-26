package opti

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	hikariarchive "hikari/internal/archive"
)

const githubReleasesAPI = "https://api.github.com/repos/optiscaler/OptiScaler/releases"

type GitHubClient struct {
	base   string
	client *http.Client
}

func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		base:   githubReleasesAPI,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitHubClient) ListReleases(ctx context.Context) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: status %d", resp.StatusCode)
	}
	var api []struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&api); err != nil {
		return nil, err
	}
	var out []Release
	for _, r := range api {
		rel := Release{Tag: r.TagName, URL: r.HTMLURL, Date: r.PublishedAt[:0]}
		if len(r.PublishedAt) >= 10 {
			rel.Date = r.PublishedAt[:10]
		}
		out = append(out, rel)
	}
	return out, nil
}

func (c *GitHubClient) DownloadAsset(ctx context.Context, tag, destDir string, progress func(Progress)) (string, error) {
	asset, err := c.assetFor(ctx, tag)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, asset.Name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asset download: status %d", resp.StatusCode)
	}

	f, err := os.CreateTemp(destDir, ".download-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	hash := sha256.New()
	written := int64(0)
	report := func() {
		if progress != nil && asset.Size > 0 {
			pct := float64(written) / float64(asset.Size) * 90
			progress(Progress{Phase: "downloading", Percent: &pct})
		}
	}
	report()
	_, err = io.Copy(io.MultiWriter(f, hash), resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	written = asset.Size
	report()

	if want := digestOf(asset.Digest); want != "" {
		got := hex.EncodeToString(hash.Sum(nil))
		if !strings.EqualFold(got, want) {
			os.Remove(tmp)
			return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", asset.Name, got[:12]+"…", want[:12]+"…")
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

func (c *GitHubClient) assetFor(ctx context.Context, tag string) (struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}, error) {
	var asset struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/tags/"+tag, nil)
	if err != nil {
		return asset, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return asset, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return asset, fmt.Errorf("github release %s: status %d", tag, resp.StatusCode)
	}
	var rel struct {
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return asset, err
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".7z") {
			return a, nil
		}
	}
	return asset, fmt.Errorf("release %s has no .7z asset", tag)
}

func digestOf(d string) string {
	if i := strings.Index(d, "sha256:"); i >= 0 {
		return d[i+len("sha256:"):]
	}
	return ""
}

func Extract7z(ctx context.Context, archive, dest string) error {
	bin := ""
	for _, cand := range []string{"7zz", "7z"} {
		if p, err := os.Stat("/usr/bin/" + cand); err == nil && !p.IsDir() {
			bin = "/usr/bin/" + cand
			break
		}
		if p, err := execLookPath(cand); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		return fmt.Errorf("7z not found; install p7zip (or 7zip) so OptiScaler releases can be extracted")
	}
	cmd := execCommand(ctx, bin, "x", "-y", "-o"+dest, archive)
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		return fmt.Errorf("7z extraction failed: %s", strings.TrimSpace(tail))
	}
	return nil
}

func ExtractTgz(archive, dest string) error {
	return hikariarchive.ExtractTgz(archive, dest)
}
