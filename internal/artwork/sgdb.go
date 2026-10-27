package artwork

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Candidate struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Source string  `json:"source"`
	Covers []Cover `json:"covers"`
}

type Cover struct {
	URL    string `json:"url"`
	Thumb  string `json:"thumb,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type Searcher struct {
	mu        sync.RWMutex
	apiKey    string
	lastSteam time.Time
	client    *http.Client

	sgdbBase  string
	steamBase string
}

func New(apiKey string) *Searcher {
	return &Searcher{
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 15 * time.Second},
		sgdbBase:  "https://www.steamgriddb.com/api/v2",
		steamBase: "https://store.steampowered.com/api",
	}
}

// SetKey swaps the SteamGridDB key without a restart, so a key entered in the UI
// starts working on the next refresh.
func (s *Searcher) SetKey(key string) {
	s.mu.Lock()
	s.apiKey = trim(key)
	s.mu.Unlock()
}

func (s *Searcher) Key() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKey
}

// paceSteam keeps store searches at a rate the endpoint answers properly; a
// library pass asks it once per game.
func (s *Searcher) paceSteam(ctx context.Context) error {
	s.mu.Lock()
	wait := time.Until(s.lastSteam.Add(700 * time.Millisecond))
	s.mu.Unlock()
	if wait > 0 {
		if err := sleepCtx(ctx, wait); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.lastSteam = time.Now()
	s.mu.Unlock()
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// StoreReachable tells a lookup that found nothing apart from the store quietly
// refusing a burst, which is what it does after a few dozen searches.
func (s *Searcher) StoreReachable(ctx context.Context) bool {
	cands, err := s.searchSteam(ctx, "portal")
	return err == nil && len(cands) > 0
}

// Check confirms a key is accepted by SteamGridDB before it is stored.
func (s *Searcher) Check(ctx context.Context, key string) error {
	if trim(key) == "" {
		return fmt.Errorf("a key is required")
	}
	var out struct {
		Success bool `json:"success"`
	}
	err := s.getJSON(ctx, s.sgdbBase+"/search/autocomplete/portal", trim(key), &out)
	if err != nil {
		return fmt.Errorf("steamgriddb rejected that key: %w", err)
	}
	return nil
}

func (s *Searcher) Search(ctx context.Context, title string) ([]Candidate, error) {
	title = trim(title)
	if title == "" {
		return nil, fmt.Errorf("a title is required")
	}
	if s.Key() != "" {
		if cands, err := s.searchSGDB(ctx, title); err == nil && len(cands) > 0 {
			return cands, nil
		}
	}
	return s.searchSteam(ctx, title)
}

type sgdbGame struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type sgdbHero struct {
	URL   string `json:"url"`
	Thumb string `json:"thumb"`
}

type sgdbGrid struct {
	URL    string `json:"url"`
	Thumb  string `json:"thumb"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (s *Searcher) searchSGDB(ctx context.Context, title string) ([]Candidate, error) {
	var games []sgdbGame
	if err := s.getJSON(ctx, s.sgdbBase+"/search/autocomplete/"+url.PathEscape(title), s.Key(), &games); err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return nil, nil
	}
	if len(games) > 4 {
		games = games[:4]
	}
	var out []Candidate
	for _, g := range games {
		var grids []sgdbGrid
		q := fmt.Sprintf("%s/grids/game/%d?dimensions=600x900", s.sgdbBase, g.ID)
		if err := s.getJSON(ctx, q, s.Key(), &grids); err != nil || len(grids) == 0 {
			continue
		}
		if len(grids) > 6 {
			grids = grids[:6]
		}
		c := Candidate{ID: fmt.Sprint(g.ID), Name: g.Name, Source: "steamgriddb"}
		for _, gr := range grids {
			c.Covers = append(c.Covers, Cover{URL: gr.URL, Thumb: gr.Thumb, Width: gr.Width, Height: gr.Height})
		}
		out = append(out, c)
	}
	return out, nil
}

type steamSearch struct {
	Items []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		TinyImage string `json:"tiny_image"`
	} `json:"items"`
}

func (s *Searcher) searchSteam(ctx context.Context, title string) ([]Candidate, error) {
	q := fmt.Sprintf("%s/storesearch/?term=%s&cc=us&l=en", s.steamBase, url.QueryEscape(title))
	var res steamSearch
	if err := s.paceSteam(ctx); err != nil {
		return nil, err
	}
	if err := s.getJSON(ctx, q, "", &res); err != nil {
		return nil, err
	}

	var out []Candidate
	for i, item := range res.Items {
		if i >= 4 {
			break
		}
		cover := fmt.Sprintf("https://cdn.akamai.steamstatic.com/steam/apps/%d/header.jpg", item.ID)
		c := Candidate{ID: fmt.Sprint(item.ID), Name: item.Name, Source: "steam",
			Covers: []Cover{{URL: cover, Thumb: item.TinyImage, Width: 460, Height: 215}}}
		if item.TinyImage != "" {
			c.Covers = append(c.Covers, Cover{URL: item.TinyImage, Thumb: item.TinyImage, Width: 231, Height: 87})
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Searcher) getJSON(ctx context.Context, rawURL, apiKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artwork search: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if apiKey != "" {
		var wrapper struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
			Errors  []string        `json:"errors"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return err
		}
		if !wrapper.Success {
			return fmt.Errorf("steamgriddb: %v", wrapper.Errors)
		}
		return json.Unmarshal(wrapper.Data, out)
	}
	return json.Unmarshal(body, out)
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
