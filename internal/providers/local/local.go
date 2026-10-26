package local

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"hikari/internal/atomicfile"
	"hikari/internal/library"
)

type Spec struct {
	Title      string   `json:"title"`
	Executable string   `json:"executable"`
	Platform   string   `json:"platform"`
	Args       []string `json:"args,omitempty"`
	Artwork    string   `json:"artwork,omitempty"`
}

type Entry struct {
	ID    string    `json:"id"`
	Spec  Spec      `json:"spec"`
	Added time.Time `json:"added"`
}

type Provider struct {
	path string
	mu   sync.Mutex
}

func New(registryPath string) *Provider { return &Provider{path: registryPath} }

func (p *Provider) Store() string { return library.StoreLocal }

func (p *Provider) List(ctx context.Context) ([]library.Game, error) {
	p.mu.Lock()
	entries, err := p.loadLocked()
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	games := make([]library.Game, 0, len(entries))
	for _, e := range entries {
		_, statErr := os.Stat(e.Spec.Executable)
		platform := normalizePlatform(e.Spec.Executable, e.Spec.Platform)
		g := library.Game{
			Store:         p.Store(),
			ID:            e.ID,
			Title:         e.Spec.Title,
			Runner:        library.RunnerLocal,
			Source:        library.SourceHikari,
			Platform:      platform,
			Installed:     statErr == nil,
			Executable:    e.Spec.Executable,
			InstallPath:   filepath.Dir(e.Spec.Executable),
			CanRunOffline: true,
			Metadata:      map[string]string{"args": strings.Join(e.Spec.Args, " ")},
		}
		if e.Spec.Artwork != "" {
			g.Artwork = library.Artwork{Cover: e.Spec.Artwork, Square: e.Spec.Artwork}
		}
		games = append(games, g)
	}
	sort.Slice(games, func(i, j int) bool { return games[i].Title < games[j].Title })
	return games, nil
}

func (p *Provider) Add(spec Spec) (library.Game, error) {
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Executable = strings.TrimSpace(spec.Executable)
	if spec.Title == "" {
		return library.Game{}, fmt.Errorf("a title is required")
	}
	if !filepath.IsAbs(spec.Executable) {
		return library.Game{}, fmt.Errorf("the executable path must be absolute")
	}
	st, err := os.Stat(spec.Executable)
	if err != nil || st.IsDir() {
		return library.Game{}, fmt.Errorf("%s is not an executable file", spec.Executable)
	}
	if spec.Platform != "Linux" && spec.Platform != "Windows" {
		return library.Game{}, fmt.Errorf("platform must be Linux or Windows")
	}
	spec.Platform = normalizePlatform(spec.Executable, spec.Platform)
	if spec.Artwork != "" && !strings.HasPrefix(spec.Artwork, "http://") && !strings.HasPrefix(spec.Artwork, "https://") {
		if _, err := os.Stat(spec.Artwork); err != nil {
			return library.Game{}, fmt.Errorf("the artwork file does not exist: %s", spec.Artwork)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	entries, err := p.loadLocked()
	if err != nil {
		return library.Game{}, err
	}
	for _, e := range entries {
		if e.Spec.Executable == spec.Executable {
			return library.Game{}, fmt.Errorf("%s is already in the library", e.Spec.Title)
		}
	}
	id := newID()
	entries = append(entries, Entry{ID: id, Spec: spec, Added: time.Now()})
	if err := p.saveLocked(entries); err != nil {
		return library.Game{}, err
	}
	g := library.Game{
		Store: p.Store(), ID: id, Title: spec.Title, Runner: library.RunnerLocal,
		Source: library.SourceHikari, Platform: spec.Platform, Installed: true,
		Executable: spec.Executable, InstallPath: filepath.Dir(spec.Executable),
		CanRunOffline: true,
	}
	if len(spec.Args) > 0 {
		g.Metadata = map[string]string{"args": strings.Join(spec.Args, " ")}
	}
	if spec.Artwork != "" {
		g.Artwork = library.Artwork{Cover: spec.Artwork, Square: spec.Artwork}
	}
	return g, nil
}

func (p *Provider) Remove(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	entries, err := p.loadLocked()
	if err != nil {
		return err
	}
	kept := entries[:0]
	found := false
	for _, e := range entries {
		if e.ID == id {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return fmt.Errorf("no local game with id %q", id)
	}
	return p.saveLocked(kept)
}

func (p *Provider) Entry(id string) (Entry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entries, err := p.loadLocked()
	if err != nil {
		return Entry{}, false
	}
	for _, e := range entries {
		if e.ID == id {
			e.Spec.Platform = normalizePlatform(e.Spec.Executable, e.Spec.Platform)
			return e, true
		}
	}
	return Entry{}, false
}

func (p *Provider) loadLocked() ([]Entry, error) {
	b, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, fmt.Errorf("local games file is corrupt: %w", err)
	}
	return entries, nil
}

func (p *Provider) saveLocked(entries []Entry) error {
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(p.path, append(b, '\n'))
}

func DetectPlatform(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".bat", ".cmd", ".msi":
		return "Windows"
	case ".sh", ".appimage":
		return "Linux"
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var head [4]byte
	n, _ := io.ReadFull(f, head[:])
	if n >= 2 && head[0] == 'M' && head[1] == 'Z' {
		return "Windows"
	}
	if n >= 4 && head[0] == 0x7f && head[1] == 'E' && head[2] == 'L' && head[3] == 'F' {
		return "Linux"
	}
	if n >= 2 && head[0] == '#' && head[1] == '!' {
		return "Linux"
	}
	return ""
}

func normalizePlatform(path, chosen string) string {
	if p := DetectPlatform(path); p != "" {
		return p
	}
	if chosen != "Linux" && chosen != "Windows" {
		return "Windows"
	}
	return chosen
}

func newID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "local-" + hex.EncodeToString(b[:])
}
