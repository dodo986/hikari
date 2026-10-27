package artwork

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"hikari/internal/library"
	"hikari/internal/providers/steam"
)

// Resolve fills the artwork slots a store left empty. Steam titles only need the
// CDN, everything else tries SteamGridDB with the user's key and then the Steam
// store search, so a GOG or Epic game still gets a portrait grid.
func (s *Searcher) Resolve(ctx context.Context, g library.Game) (library.Artwork, error) {
	out := g.Artwork
	if out.Cover != "" && out.Square != "" && out.Logo != "" {
		return out, nil
	}
	if g.Store == library.StoreSteam && g.ID != "" {
		fill(&out, steam.SteamCDN(g.ID))
		return out, nil
	}
	if s.Key() != "" {
		if cands, err := s.searchSGDB(ctx, g.Title); err == nil && len(cands) > 0 {
			best := pick(g.Title, cands)
			if best != nil && len(best.Covers) > 0 {
				if out.Cover == "" {
					out.Cover = best.Covers[0].URL
				}
				logo := s.sgdbLogo(ctx, best.ID)
				if out.Logo == "" {
					out.Logo = logo
				}
				if out.Square == "" {
					out.Square = s.sgdbHero(ctx, best.ID)
				}
				fill(&out, steam.SteamCDN(best.ID))
				return out, nil
			}
		}
	}
	for _, query := range queries(g.Title) {
		cands, err := s.searchSteam(ctx, query)
		if err != nil {
			return out, err
		}
		if best := pick(g.Title, cands); best != nil {
			fill(&out, steam.SteamCDN(best.ID))
			return out, nil
		}
	}
	return out, fmt.Errorf("no store page matched %q", g.Title)
}

// queries widens a search that finds nothing: store search does not cope with
// subtitles or trademark marks, so the plain name is tried as well.
func queries(title string) []string {
	out := []string{title}
	clean := strings.TrimSpace(strings.NewReplacer("\u2122", "", "\u00ae", "", "\u00a9", "").Replace(title))
	if clean != title && clean != "" {
		out = append(out, clean)
	}
	var cuts []int
	for _, sep := range []string{" - ", ": ", " \u2013 "} {
		for at := 0; ; {
			j := strings.Index(clean[at:], sep)
			if j < 0 {
				break
			}
			if at+j > 2 {
				cuts = append(cuts, at+j)
			}
			at += j + len(sep)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(cuts)))
	for _, at := range cuts {
		out = append(out, strings.TrimSpace(clean[:at]))
	}
	return out
}

func fill(into *library.Artwork, from library.Artwork) {
	if into.Cover == "" {
		into.Cover = from.Cover
	}
	if into.Square == "" {
		into.Square = from.Square
	}
	if into.Logo == "" {
		into.Logo = from.Logo
	}
}

func (s *Searcher) sgdbLogo(ctx context.Context, gameID string) string {
	var logos []sgdbGrid
	q := s.sgdbBase + "/logos/game/" + gameID
	if err := s.getJSON(ctx, q, s.Key(), &logos); err != nil || len(logos) == 0 {
		return ""
	}
	return logos[0].URL
}

func (s *Searcher) sgdbHero(ctx context.Context, gameID string) string {
	var heroes []sgdbHero
	q := s.sgdbBase + "/heroes/game/" + gameID + "?dimensions=1920x620"
	if err := s.getJSON(ctx, q, s.Key(), &heroes); err != nil || len(heroes) == 0 {
		return ""
	}
	return heroes[0].URL
}

// subtitleOf reports whether the candidate is the wanted title plus a subtitle,
// which is how stores rename editions without it being a different game.
func subtitleOf(candidate, title string) bool {
	c, t := strings.ToLower(strings.TrimSpace(candidate)), strings.ToLower(strings.TrimSpace(title))
	if !strings.HasPrefix(c, t) {
		return false
	}
	rest := strings.TrimSpace(c[len(t):])
	for _, sep := range []string{":", " - ", " \u2013 ", " \u2014 "} {
		if strings.HasPrefix(rest, sep) {
			return true
		}
	}
	return false
}

// pick returns the candidate that actually names this game, so "Doom" never
// gets the art of "Doom Eternal" or a soundtrack page.
func pick(title string, cands []Candidate) *Candidate {
	want := normalizeTitle(title)
	if want == "" {
		return nil
	}
	type scored struct {
		c     Candidate
		score float64
	}
	var all []scored
	for _, c := range cands {
		score, ok := matchScore(want, normalizeTitle(c.Name))
		if !ok && subtitleOf(c.Name, title) {
			score, ok = 0.8, true
		}
		if !ok {
			continue
		}
		all = append(all, scored{c, score})
	}
	if len(all) == 0 {
		return nil
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	best := all[0]
	return &best.c
}

var noise = map[string]bool{
	"the": true, "of": true, "and": true, "a": true, "an": true,
	"edition": true, "definitive": true, "deluxe": true, "ultimate": true,
	"complete": true, "goty": true, "remastered": true, "remake": true,
	"soundtrack": true, "demo": true, "dlc": true, "artbook": true,
	"bundle": true, "pack": true, "classic": true, "enhanced": true,
	"game": true, "launcher": true, "final": true, "cut": true,
	"director": true, "directors": true, "anniversary": true, "gold": true,
	"redux": true, "digital": true, "hd": true, "vr": true,
}

func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(":", " ", "-", " ", "'", "", "’", "", "™", "", "®", "", "©", "",
		"(", " ", ")", " ", "[", " ", "]", " ", "_", " ").Replace(s)
	fields := strings.Fields(s)
	kept := fields[:0]
	for _, f := range fields {
		f = strings.Trim(f, " .,!")
		if f == "" {
			continue
		}
		kept = append(kept, f)
	}
	return strings.Join(kept, " ")
}

// matchScore compares a wanted title with a candidate name and reports whether
// the candidate is the same game at all.
func matchScore(want, got string) (float64, bool) {
	if want == "" || got == "" {
		return 0, false
	}
	if want == got {
		return 1.5, true
	}
	for _, bad := range []string{"soundtrack", "artbook", "demo version", "dedicated server"} {
		if strings.Contains(got, bad) {
			return 0, false
		}
	}
	a, b := tokens(want), tokens(got)
	if len(a) == 0 || len(b) == 0 {
		return 0, false
	}
	shared := 0
	for t := range a {
		if b[t] {
			shared++
		}
	}
	if shared == len(a) && shared == len(b) {
		return 0.9, true
	}
	union := len(a) + len(b) - shared
	score := float64(shared) / float64(union)
	if score < 0.7 {
		return score, false
	}
	if strings.Contains(got, want) || strings.Contains(want, got) {
		score += 0.05
	}
	return score, true
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Fields(s) {
		if len(f) < 2 || noise[f] {
			continue
		}
		out[f] = true
	}
	return out
}
