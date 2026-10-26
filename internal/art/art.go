package art

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Getter interface {
	Get(url string) (resp *http.Response, err error)
}

type Cache struct {
	dir           string
	client        Getter
	maxImageBytes int64
	maxTotalBytes int64
}

const (
	DefaultMaxImage = 5 << 20
	DefaultMaxTotal = 200 << 20
)

func New(dir string) *Cache {
	return &Cache{
		dir:           dir,
		client:        &http.Client{Timeout: 30 * time.Second},
		maxImageBytes: DefaultMaxImage,
		maxTotalBytes: DefaultMaxTotal,
	}
}

func NewWithGetter(dir string, g Getter) *Cache {
	c := New(dir)
	c.client = g
	return c
}

func (c *Cache) PathFor(url string) string {
	if url == "" || !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		return ""
	}
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:16])+extOf(url))
}

func (c *Cache) Seed(url string, r io.Reader) error {
	p := c.PathFor(url)
	if p == "" {
		return fmt.Errorf("not an http(s) artwork URL")
	}
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (c *Cache) Has(url string) bool {
	p := c.PathFor(url)
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func (c *Cache) Fetch(ctx context.Context, url string) (string, error) {
	p := c.PathFor(url)
	if p == "" {
		return "", fmt.Errorf("not an http(s) artwork URL")
	}
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Get(req.URL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("artwork fetch status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(c.dir, ".fetch-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, c.maxImageBytes+1))
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if n > c.maxImageBytes {
		os.Remove(tmpName)
		return "", fmt.Errorf("artwork exceeds %d bytes", c.maxImageBytes)
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	c.evictIfNeeded()
	return p, nil
}

func (c *Cache) evictIfNeeded() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type file struct {
		path    string
		size    int64
		modTime time.Time
	}
	var files []file
	var total int64
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".fetch-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{filepath.Join(c.dir, e.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= c.maxTotalBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for _, f := range files {
		if total <= c.maxTotalBytes {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

func extOf(url string) string {
	base := url
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return strings.ToLower(filepath.Ext(base))
	}
	return ".jpg"
}
