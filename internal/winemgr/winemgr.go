package winemgr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Version struct {
	Name       string `json:"name"`
	Bin        string `json:"bin"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	Lib        string `json:"lib,omitempty"`
	Lib32      string `json:"lib32,omitempty"`
	Wineserver string `json:"wineserver,omitempty"`
}

type Release struct {
	Tag     string `json:"tag"`
	Name    string `json:"name"`
	Variant string `json:"variant"`
	Kind    string `json:"kind"`
	Date    string `json:"date"`
	PageURL string `json:"url"`
	Size    int64  `json:"size"`

	download    string
	checksumURL string
}

const (
	geProtonAPI    = "https://api.github.com/repos/GloriousEggroll/proton-ge-custom/releases"
	cachyProtonAPI = "https://api.github.com/repos/CachyOS/proton-cachyos/releases"
	githubAPIRoot  = "https://api.github.com"
)

type Manager struct {
	wineDir   string
	protonDir string
	home      string

	steamRoots []string

	client  *http.Client
	apiBase string

	mu   sync.Mutex
	rels []Release
}

func New(dataDir string) *Manager {
	home, _ := os.UserHomeDir()
	return &Manager{
		wineDir:   filepath.Join(dataDir, "tools", "wine"),
		protonDir: filepath.Join(dataDir, "tools", "proton"),
		home:      home,
		client:    &http.Client{Timeout: 60 * time.Second},
		apiBase:   githubAPIRoot,
	}
}

func NewForTest(home, dataDir string, steamRoots []string) *Manager {
	m := New(dataDir)
	m.home = home
	m.steamRoots = steamRoots
	return m
}

func (m *Manager) List(ctx context.Context) ([]Version, error) {
	var out []Version
	seen := map[string]bool{}
	add := func(v Version) {
		if v.Bin == "" {
			return
		}
		if _, err := os.Stat(v.Bin); err != nil {
			return
		}
		key := v.Bin
		if resolved, err := filepath.EvalSymlinks(v.Bin); err == nil {
			key = resolved
		}
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, v)
	}

	for _, dir := range entries(m.wineDir) {
		bin := filepath.Join(m.wineDir, dir, "bin", "wine")
		add(wineVersion(bin, dir, "hikari"))
	}
	for _, dir := range entries(m.protonDir) {
		bin := filepath.Join(m.protonDir, dir, "proton")
		add(Version{Name: dir, Bin: bin, Type: "proton", Source: "hikari"})
	}

	lutris := filepath.Join(m.home, ".local", "share", "lutris", "runners", "wine")
	for _, dir := range entries(lutris) {
		bin := filepath.Join(lutris, dir, "bin", "wine")
		add(wineVersion(bin, dir, "lutris"))
	}

	for _, root := range m.steamLibraryRoots() {
		for _, ct := range []string{
			filepath.Join(root, "compatibilitytools.d"),
			filepath.Join(root, "root", "compatibilitytools.d"),
		} {
			for _, dir := range entries(ct) {
				if strings.HasPrefix(dir, "UMU-Latest") {
					continue
				}
				add(Version{
					Name: dir, Bin: filepath.Join(ct, dir, "proton"),
					Type: "proton", Source: "steam",
				})
			}
		}
		for _, dir := range entries(filepath.Join(root, "steamapps", "common")) {
			bin := filepath.Join(root, "steamapps", "common", dir, "proton")
			if _, err := os.Stat(bin); err == nil {
				add(Version{Name: dir, Bin: bin, Type: "proton", Source: "steam"})
			}
		}
	}

	if bin, err := exec.LookPath("wine"); err == nil {
		add(wineVersion(bin, "system wine", "system"))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "proton"
		}
		return out[i].Name > out[j].Name
	})
	return out, nil
}

func wineVersion(bin, name, source string) Version {
	dir := filepath.Dir(bin)
	v := Version{Name: name, Bin: bin, Type: "wine", Source: source}
	if st, err := os.Stat(filepath.Join(dir, "..", "lib")); err == nil && st.IsDir() {
		v.Lib32 = filepath.Clean(filepath.Join(dir, "..", "lib"))
	}
	if st, err := os.Stat(filepath.Join(dir, "..", "lib64")); err == nil && st.IsDir() {
		v.Lib = filepath.Clean(filepath.Join(dir, "..", "lib64"))
	}
	if _, err := os.Stat(filepath.Join(dir, "wineserver")); err == nil {
		v.Wineserver = filepath.Join(dir, "wineserver")
	}
	return v
}

func (m *Manager) steamLibraryRoots() []string {
	if m.steamRoots != nil {
		return m.steamRoots
	}
	home := m.home
	roots := []string{
		"/usr/share/steam",
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
	}
	for _, root := range append([]string{}, roots...) {
		roots = append(roots, extraSteamLibraries(root)...)
	}
	return roots
}

func extraSteamLibraries(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "\"path\"") {
			continue
		}
		parts := strings.Split(t, "\"")
		for _, p := range parts {
			if strings.HasPrefix(p, "/") {
				out = append(out, p)
			}
		}
	}
	return out
}

func entries(dir string) []string {
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range list {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

type ghRelease struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func (m *Manager) Releases(ctx context.Context) ([]Release, error) {
	var all []Release
	var firstErr error
	for _, src := range []struct {
		api     string
		variant string
	}{
		{m.apiBase + "/repos/GloriousEggroll/proton-ge-custom/releases", "GE-Proton"},
		{m.apiBase + "/repos/CachyOS/proton-cachyos/releases", "Proton-CachyOS"},
	} {
		rels, err := m.fetchReleases(ctx, src.api, src.variant)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		all = append(all, rels...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	m.mu.Lock()
	m.rels = all
	m.mu.Unlock()
	return all, nil
}

func (m *Manager) fetchReleases(ctx context.Context, api, variant string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"?per_page=30", nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wine releases: status %d", resp.StatusCode)
	}
	var raw []ghRelease
	if err := decodeJSON(resp.Body, &raw); err != nil {
		return nil, err
	}
	var out []Release
	for _, r := range raw {
		rel := Release{
			Tag: r.TagName, Name: r.TagName, Variant: variant,
			Kind: "proton", PageURL: r.HTMLURL,
		}
		if len(r.PublishedAt) >= 10 {
			rel.Date = r.PublishedAt[:10]
		}
		for _, a := range r.Assets {
			if !isCompatibleAsset(variant, a.Name) {
				continue
			}
			rel.download = a.URL
			rel.Size = a.Size
			candidates := sha512Names(a.Name)
			for _, c := range r.Assets {
				for _, want := range candidates {
					if c.Name == want {
						rel.checksumURL = c.URL
					}
				}
			}
			break
		}
		if rel.download != "" {
			out = append(out, rel)
		}
	}
	return out, nil
}

func isCompatibleAsset(variant, name string) bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64") {
		return false
	}
	switch variant {
	case "Proton-CachyOS":
		return strings.HasSuffix(lower, "-x86_64.tar.xz")
	default:
		return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.xz")
	}
}

func sha512Names(archive string) []string {
	names := []string{archive + ".sha512sum"}
	for _, suffix := range []string{".tar.gz", ".tar.xz"} {
		if strings.HasSuffix(archive, suffix) {
			names = append(names, strings.TrimSuffix(archive, suffix)+".sha512sum")
		}
	}
	return names
}

func (m *Manager) Download(ctx context.Context, tag string, progress func(DownloadProgress)) (string, error) {
	rel, err := m.releaseFor(ctx, tag)
	if err != nil {
		return "", err
	}
	destDir := m.protonDir
	sub := rel.Name
	if rel.Kind == "wine" {
		destDir = m.wineDir
	}
	target := filepath.Join(destDir, sub)
	if _, err := os.Stat(target); err == nil {
		return sub, fmt.Errorf("%s is already installed in %s", sub, destDir)
	}

	checksum := ""
	if rel.checksumURL != "" {
		checksum, err = m.fetchChecksum(ctx, rel.checksumURL)
		if err != nil {
			return "", err
		}
	}

	archive, err := m.downloadVerified(ctx, rel, checksum, progress)
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)

	if progress != nil {
		progress(DownloadProgress{Phase: "extracting", Name: sub})
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(destDir, ".extract-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := extractStrip1(ctx, archive, tmp); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	if progress != nil {
		pct := 100.0
		progress(DownloadProgress{Phase: "done", Percent: &pct, Name: sub, Done: true})
	}
	return sub, nil
}

type DownloadProgress struct {
	Phase   string   `json:"phase"`
	Percent *float64 `json:"percent"`
	Name    string   `json:"name,omitempty"`
	Done    bool     `json:"done"`
}

func (m *Manager) releaseFor(ctx context.Context, tag string) (Release, error) {
	m.mu.Lock()
	for _, r := range m.rels {
		if r.Tag == tag {
			m.mu.Unlock()
			return r, nil
		}
	}
	m.mu.Unlock()
	rels, err := m.Releases(ctx)
	if err != nil {
		return Release{}, err
	}
	for _, r := range rels {
		if r.Tag == tag {
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("no downloadable release with tag %q", tag)
}

func (m *Manager) fetchChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum fetch: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum file")
	}
	return strings.ToLower(fields[0]), nil
}

func (m *Manager) downloadVerified(ctx context.Context, rel Release, want string, progress func(DownloadProgress)) (string, error) {
	if err := os.MkdirAll(m.protonDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(m.protonDir, ".download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.download, nil)
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("wine download: status %d", resp.StatusCode)
	}

	hash := newSHA512()
	written := int64(0)
	report := func(done bool) {
		if progress == nil {
			return
		}
		var pct *float64
		if rel.Size > 0 {
			p := float64(written) / float64(rel.Size) * 100
			if p > 99 {
				p = 99
			}
			pct = &p
		}
		progress(DownloadProgress{Phase: "downloading", Percent: pct, Name: rel.Name, Done: done})
	}
	report(false)

	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				tmp.Close()
				os.Remove(tmpPath)
				return "", werr
			}
			hash.Write(buf[:n])
			written += int64(n)
			report(false)
		}
		if rerr != nil {
			if rerr != io.EOF {
				tmp.Close()
				os.Remove(tmpPath)
				return "", fmt.Errorf("wine download interrupted: %w", rerr)
			}
			break
		}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	if want != "" {
		got := hash.hex()
		if got != want {
			os.Remove(tmpPath)
			return "", fmt.Errorf("checksum mismatch for %s (got %s…, want %s…)", rel.Name, got[:12], want[:12])
		}
	}
	return tmpPath, nil
}

func (m *Manager) Remove(ctx context.Context, name string) error {
	for _, dir := range []string{m.wineDir, m.protonDir} {
		target := filepath.Join(dir, name)
		if st, err := os.Stat(target); err == nil && st.IsDir() {
			return os.RemoveAll(target)
		}
	}
	return fmt.Errorf("%q is not a Hikari-installed version", name)
}

func (m *Manager) Installed(name string) bool {
	for _, dir := range []string{m.wineDir, m.protonDir} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func decodeJSON(r io.Reader, v any) error {
	b, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func readAll(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
