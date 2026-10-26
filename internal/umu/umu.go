package umu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const APIBase = "https://umu.openwinecomponents.org/umu_api.php"

func storeParam(store string) string {
	switch store {
	case "epic":
		return "egs"
	case "gog":
		return "gog"
	case "amazon":
		return "amazon"
	}
	return ""
}

type cache struct {
	IDs    map[string]string `json:"ids"`
	Misses map[string]bool   `json:"misses"`
}

type Resolver struct {
	path   string
	base   string
	client *http.Client
}

func NewResolver(cachePath string) *Resolver {
	return &Resolver{
		path:   cachePath,
		base:   APIBase,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *Resolver) Resolve(ctx context.Context, store, appID string) (string, error) {
	sp := storeParam(store)
	if sp == "" || appID == "" {
		return "", nil
	}
	c := r.load()
	key := sp + "_" + appID
	if id, ok := c.IDs[key]; ok {
		return id, nil
	}
	if c.Misses[key] {
		return "", nil
	}

	id, err := r.query(ctx, sp, appID)
	if err != nil {
		return "", err
	}
	if id == "" {
		c.Misses[key] = true
	} else {
		c.IDs[key] = id
	}
	r.save(c)
	return id, nil
}

func (r *Resolver) query(ctx context.Context, store, appID string) (string, error) {
	url := fmt.Sprintf("%s?codename=%s&store=%s", r.base, appID, store)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("umu api status %d", resp.StatusCode)
	}
	var games []struct {
		UmuID string `json:"umu_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&games); err != nil {
		return "", err
	}
	if len(games) == 0 {
		return "", nil
	}
	return games[0].UmuID, nil
}

func (r *Resolver) load() cache {
	c := cache{IDs: map[string]string{}, Misses: map[string]bool{}}
	if b, err := os.ReadFile(r.path); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func (r *Resolver) save(c cache) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(r.path, append(b, '\n'), 0o600)
}
