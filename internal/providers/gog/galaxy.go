package gog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hikari/internal/library"
)

type GalaxyClient struct {
	libBase string
	dbBase  string
	client  *http.Client
}

func NewGalaxyClient() *GalaxyClient {
	return &GalaxyClient{
		libBase: "https://galaxy-library.gog.com",
		dbBase:  "https://gamesdb.gog.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type Credentials struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
}

type galaxyLibraryEntry struct {
	PlatformID  string `json:"platform_id"`
	ExternalID  string `json:"external_id"`
	Origin      string `json:"origin"`
	Owned       bool   `json:"owned"`
	Certificate string `json:"certificate"`
}

func (c *GalaxyClient) Library(ctx context.Context, creds Credentials) ([]galaxyLibraryEntry, error) {
	var all []galaxyLibraryEntry
	pageToken := ""
	for {
		url := fmt.Sprintf("%s/users/%s/releases?limit=100", c.libBase, creds.UserID)
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}
		var page struct {
			Items         []galaxyLibraryEntry `json:"items"`
			NextPageToken string               `json:"next_page_token"`
		}
		if err := c.getJSON(ctx, url, creds.AccessToken, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if page.NextPageToken == "" {
			return all, nil
		}
		pageToken = page.NextPageToken
	}
}

type gamesdbRelease struct {
	Type       string            `json:"type"`
	ExternalID string            `json:"external_id"`
	Summary    map[string]string `json:"summary"`
	Game       struct {
		VisibleInLibrary bool              `json:"visible_in_library"`
		Title            map[string]string `json:"title"`
		Developers       []struct {
			Name string `json:"name"`
		} `json:"developers"`
		Logo struct {
			URLFormat string `json:"url_format"`
		} `json:"logo"`
		VerticalCover struct {
			URLFormat string `json:"url_format"`
		} `json:"vertical_cover"`
		SquareIcon struct {
			URLFormat string `json:"url_format"`
		} `json:"square_icon"`
		Background struct {
			URLFormat string `json:"url_format"`
		} `json:"background"`
		Genres []struct {
			Name map[string]string `json:"name"`
		} `json:"genres"`
	} `json:"game"`
	Title2                    map[string]string `json:"title"`
	SupportedOperatingSystems []struct {
		Slug string `json:"slug"`
	} `json:"supported_operating_systems"`
}

func (c *GalaxyClient) GameMeta(ctx context.Context, creds Credentials, externalID, certificate string) (*gamesdbRelease, error) {
	url := fmt.Sprintf("%s/platforms/gog/external_releases/%s", c.dbBase, externalID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	if certificate != "" {
		req.Header.Set("X-GOG-Library-Cert", certificate)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gamesdb: status %d for %s", resp.StatusCode, externalID)
	}
	var rel gamesdbRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (c *GalaxyClient) getJSON(ctx context.Context, url, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("galaxy library: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (r *gamesdbRelease) toGame() (library.Game, bool) {
	if r.Type != "game" && r.Type != "mod" {
		return library.Game{}, false
	}
	if !r.Game.VisibleInLibrary {
		return library.Game{}, false
	}
	g := library.Game{
		Store:  library.StoreGOG,
		ID:     r.ExternalID,
		Runner: library.RunnerGogdl,
		Source: library.SourceHikari,
		Title:  firstNonEmpty(r.Title2["*"], r.Game.Title["*"]),
	}
	if g.Title == "" {
		return library.Game{}, false
	}
	for _, d := range r.Game.Developers {
		if g.Developer != "" {
			g.Developer += ", "
		}
		g.Developer += d.Name
	}
	logo := fmtURL(r.Game.Logo.URLFormat, "jpg")
	wide := fmtURL(r.Game.Background.URLFormat, "webp")
	g.Artwork = library.Artwork{
		Cover:  orStr(fmtURL(r.Game.VerticalCover.URLFormat, "jpg"), wide),
		Square: wide,
		Logo:   logo,
		Icon:   fmtURL(r.Game.SquareIcon.URLFormat, "jpg"),
	}
	for _, os := range r.SupportedOperatingSystems {
		switch os.Slug {
		case "linux":
			g.Platform = "Linux"
		case "osx":
			if g.Platform == "" {
				g.Platform = "Mac"
			}
		}
	}
	if g.Platform == "" {
		g.Platform = "Windows"
	}
	g.CanRunOffline = true
	return g, true
}

func fmtURL(format, ext string) string {
	if format == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(format, "{formatter}", ""), "{ext}", ext)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func orStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
