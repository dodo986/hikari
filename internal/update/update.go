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
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultRepo = "dodo986/hikari"

type Info struct {
	Current   string `json:"current"`
	Commit    string `json:"commit,omitempty"`
	Latest    string `json:"latest,omitempty"`
	Available bool   `json:"available"`
	Behind    int    `json:"behind,omitempty"`
	Branch    string `json:"branch,omitempty"`
	URL       string `json:"url,omitempty"`
	Notes     string `json:"notes,omitempty"`

	Sha string `json:"-"`
}

// Checker tracks the source branch instead of releases, so a build is out of
// date as soon as a commit lands rather than when somebody publishes.
type Checker struct {
	Repo     string
	Branch   string
	Base     string
	Codeload string
	Client   *http.Client
}

func New(repo string) *Checker {
	if repo == "" {
		repo = defaultRepo
	}
	return &Checker{
		Repo:     repo,
		Branch:   "main",
		Base:     "https://api.github.com",
		Codeload: "https://codeload.github.com",
		Client:   &http.Client{Timeout: 20 * time.Second},
	}
}

type commit struct {
	Sha    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	HTMLURL string `json:"html_url"`
}

func (c *Checker) commits(ctx context.Context, perPage int) ([]commit, error) {
	url := fmt.Sprintf("%s/repos/%s/commits?sha=%s&per_page=%d", c.Base, c.Repo, c.Branch, perPage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("commit check: status %d", resp.StatusCode)
	}
	var out []commit
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("commit check: %s has no commits", c.Branch)
	}
	return out, nil
}

// Check reports how far the running build is behind the branch head. A build
// without a stamped commit cannot answer, so it only reports the head.
func (c *Checker) Check(ctx context.Context, current, commit string) (Info, error) {
	info := Info{Current: current, Commit: commit, Branch: c.Branch}
	history, err := c.commits(ctx, 60)
	if err != nil {
		return info, err
	}
	head := history[0]
	info.Latest, info.Sha = short(head.Sha), head.Sha
	info.URL = fmt.Sprintf("https://github.com/%s/compare/%s...%s", c.Repo, commit, c.Branch)
	if commit == "" {
		info.Notes = subject(head)
		return info, nil
	}
	if sameCommit(commit, head.Sha) {
		return info, nil
	}
	ahead := subjects(history[:1])
	for i, cm := range history {
		if sameCommit(commit, cm.Sha) {
			info.Behind = i
			ahead = subjects(history[:i])
			break
		}
	}
	if info.Behind == 0 && !sameCommit(commit, head.Sha) {
		info.Behind = -1
	}
	info.Available = true
	info.Notes = ahead
	return info, nil
}

func subjects(list []commit) string {
	var lines []string
	for _, cm := range list {
		lines = append(lines, "- "+subject(cm))
	}
	return strings.Join(lines, "\n")
}

func subject(cm commit) string {
	line := strings.TrimSpace(cm.Commit.Message)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

func sameCommit(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if len(a) < 7 || len(b) < 7 {
		return a == b
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// VersionAtLeast reports whether a is b or newer, so building an older tree over
// a newer install can be refused.
func VersionAtLeast(a, b string) bool {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return true
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// Source fetches the tree of one commit and returns the archive path and its
// digest, so an install can build what the branch actually holds.
func (c *Checker) Source(ctx context.Context, info Info, dest string, progress func(done int64)) (string, string, error) {
	if info.Sha == "" {
		return "", "", fmt.Errorf("the update check did not name a commit to build")
	}
	url := fmt.Sprintf("%s/%s/tar.gz/%s", c.Codeload, c.Repo, info.Sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("fetching the source: status %d", resp.StatusCode)
	}
	file := filepath.Join(dest, fmt.Sprintf("source-%s.tar.gz", short(info.Sha)))
	f, err := os.Create(file)
	if err != nil {
		return "", "", err
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
				return "", "", werr
			}
			hash.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(file)
			return "", "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", "", err
	}
	return file, hex.EncodeToString(hash.Sum(nil)), nil
}
