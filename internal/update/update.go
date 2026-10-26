package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultRepo = "dodo986/hikari"

type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	Notes     string `json:"notes,omitempty"`

	Asset    string `json:"-"`
	Checksum string `json:"-"`
	Size     int64  `json:"-"`
}

// Checker looks up the latest release. Format selects the asset kind:
// "appimage" for the single-file build, empty for the tarball.
type Checker struct {
	Repo   string
	Base   string
	Format string
	Client *http.Client
}

func New(repo string) *Checker {
	if repo == "" {
		repo = defaultRepo
	}
	return &Checker{
		Repo:   repo,
		Base:   "https://api.github.com",
		Client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Checker) Check(ctx context.Context, current string) (Info, error) {
	info := Info{Current: current}
	if !regexp.MustCompile(`^[\w.-]+/[\w.-]+$`).MatchString(c.Repo) {
		return info, fmt.Errorf("invalid update repo %q", c.Repo)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/repos/"+c.Repo+"/releases/latest", nil)
	if err != nil {
		return info, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("release check: status %d", resp.StatusCode)
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		URL    string `json:"html_url"`
		Body   string `json:"body"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = rel.Tag
	info.URL = rel.URL
	info.Notes = rel.Body
	if compareVersions(rel.Tag, current) <= 0 {
		return info, nil
	}
	suffix, kind := ".tar.gz", "tar.gz"
	if strings.EqualFold(c.Format, "appimage") {
		suffix, kind = ".appimage", "AppImage"
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "hikari-") && strings.HasSuffix(name, suffix) && archMatch(name) {
			info.Asset, info.Size = a.URL, a.Size
			break
		}
	}
	if info.Asset == "" {
		return info, fmt.Errorf("release %s has no hikari-*.%s asset", rel.Tag, kind)
	}
	want := path.Base(info.Asset) + ".sha256"
	for _, a := range rel.Assets {
		if a.Name == want {
			info.Checksum = a.URL
			break
		}
	}
	if info.Checksum == "" {
		return info, fmt.Errorf("release %s has no %s", rel.Tag, want)
	}
	info.Available = true
	return info, nil
}

func archMatch(name string) bool {
	arch := map[string]string{"amd64": "x86_64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" || strings.Contains(name, arch) {
		return true
	}
	return !strings.Contains(name, "x86_64") && !strings.Contains(name, "arm64")
}

func compareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func (c *Checker) Download(ctx context.Context, info Info, dest string, progress func(done, total int64)) (string, error) {
	if info.Asset == "" || info.Checksum == "" {
		return "", fmt.Errorf("release is missing its asset or checksum")
	}
	want, err := c.checksum(ctx, info.Checksum)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.Asset, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}
	file := filepath.Join(dest, path.Base(info.Asset))
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(file)
				return "", werr
			}
			hash.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, info.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(file)
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(file)
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != want {
		os.Remove(file)
		return "", fmt.Errorf("checksum mismatch (got %s…, want %s…)", got[:12], want[:12])
	}
	return file, nil
}

func (c *Checker) checksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum: status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, field := range strings.Fields(string(b)) {
		if len(field) == 64 {
			if _, err := hex.DecodeString(field); err == nil {
				return strings.ToLower(field), nil
			}
		}
	}
	return "", fmt.Errorf("checksum file has no sha256")
}
