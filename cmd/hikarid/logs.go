package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/launch"
	"hikari/internal/library"
	"hikari/internal/mods"
)

func (d *daemon) registerLogs(srv *api.Server) {
	srv.Register("logs.tail", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Lines int `json:"lines"`
		}
		_ = json.Unmarshal(params, &p)
		n := p.Lines
		if n <= 0 || n > 500 {
			n = 120
		}
		out := d.logRing
		if out == nil {
			return map[string]any{"path": d.logPath, "lines": []string{}}, nil
		}
		return map[string]any{"path": d.logPath, "lines": out.Lines(n)}, nil
	})

	srv.Register("logs.report", func(c context.Context, params json.RawMessage) (any, error) {
		return map[string]any{"text": d.report(c)}, nil
	})

	srv.Register("game.log", func(c context.Context, params json.RawMessage) (any, error) {
		var p struct {
			Store string `json:"store"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(params, &p); err != nil || p.Store == "" || p.ID == "" {
			return nil, api.Errorf(api.CodeInvalidParams, "params must be {store, id}")
		}
		rec, ok := d.games.LastRun(p.Store, p.ID)
		if !ok {
			return map[string]any{
				"store": p.Store, "id": p.ID, "running": false,
				"lines": []string{}, "exit_code": 0,
			}, nil
		}
		return logResponse{RunLog: rec, ModStop: d.stopNote(c, p.Store, p.ID, rec)}, nil
	})
}

// logResponse is a run record plus, for a launch that ended at once, the loader
// log sentence that explains it.
type logResponse struct {
	launch.RunLog
	ModStop *mods.StopNote `json:"mod_stop,omitempty"`
}

// shortRun is how soon a finished launch still counts as "it never got going".
const shortRun = 30 * time.Second

// stopNote reads the loader logs of a launch that ended at once, so the answer
// to "why did nothing happen" is in the log the player is already looking at.
func (d *daemon) stopNote(ctx context.Context, store, id string, rec launch.RunLog) *mods.StopNote {
	// A game the player stopped is not a mystery, and a long session that
	// crashed says nothing about the loaders it got past.
	if rec.Running || rec.Canceled || rec.State != launch.StateCrashed && time.Since(rec.StartedAt) > shortRun {
		return nil
	}
	g, err := d.library.Get(ctx, store, id)
	if err != nil || g.InstallPath == "" {
		return nil
	}
	end := rec.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	note, err := d.mods.StopNote(ctx, g.InstallPath, rec.StartedAt, end)
	if err != nil {
		d.log.Debug("mod stop note unavailable", "game", store+"/"+id, "err", err)
		return nil
	}
	return note
}

func (d *daemon) report(ctx context.Context) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Hikari %s\n", d.version)
	fmt.Fprintf(&b, "Kernel: %s %s (%s)\n", runtime.GOOS, runtime.GOARCH, kernelVersion())
	fmt.Fprintf(&b, "Session: %s\n", sessionType())
	fmt.Fprintf(&b, "Socket: %s\n", config.SocketPath())
	fmt.Fprintf(&b, "Data: %s\n", d.paths.DataDir)
	fmt.Fprintf(&b, "Log: %s\n", d.logPath)
	fmt.Fprintf(&b, "Uptime: %s\n", timeSince(d.started))

	fmt.Fprintf(&b, "\nEngines:\n")
	statuses := d.eng.Status(ctx)
	names := make([]string, 0, len(statuses))
	for name := range statuses {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		st := statuses[name]
		if !st.Available {
			fmt.Fprintf(&b, "  %-10s missing\n", name)
			continue
		}
		version := st.Version
		if strings.HasPrefix(strings.ToLower(version), "usage:") {
			version = "installed"
		}
		fmt.Fprintf(&b, "  %-10s %-16s %s\n", name, version, st.Path)
	}

	if d.wine != nil {
		fmt.Fprintf(&b, "\nWine/Proton builds:\n")
		if versions, err := d.wine.List(ctx); err == nil {
			for _, v := range versions {
				fmt.Fprintf(&b, "  %-7s %-28s %s\n", v.Type, v.Name, v.Bin)
			}
		} else {
			fmt.Fprintf(&b, "  (list failed: %v)\n", err)
		}
	}

	if d.mods != nil {
		probe, cancel := context.WithTimeout(ctx, 10*time.Second)
		modsStatus := d.mods.Status(probe)
		cancel()
		if modsStatus.Available {
			fmt.Fprintf(&b, "\nMods backend: ready (%s)\n", modsStatus.Source)
		} else {
			fmt.Fprintf(&b, "\nMods backend: unavailable — %s\n", modsStatus.Reason)
		}
	}

	def := d.settings.Defaults
	fmt.Fprintf(&b, "\nLaunch defaults: wine_type=%s prefix=%s\n",
		def.WineVersion.Type, def.WinePrefix)
	games, _ := d.library.List(ctx, "", false)
	fmt.Fprintf(&b, "Games: %d\n", len(games))
	if fails := d.localFailures(games); len(fails) > 0 {
		fmt.Fprintf(&b, "\nLocal games whose last launch failed:\n")
		for _, f := range fails {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	if d.logRing != nil {
		fmt.Fprintf(&b, "\nRecent log:\n")
		for _, line := range d.logRing.Lines(80) {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

func (d *daemon) localFailures(games []library.Game) []string {
	var out []string
	for _, g := range games {
		if g.Store != library.StoreLocal {
			continue
		}
		rec, ok := d.games.LastRun(g.Store, g.ID)
		if !ok || rec.Error == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s (%s): %s", g.Title, g.ID, rec.Error))
	}
	return out
}

func kernelVersion() string {
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		return strings.TrimSpace(string(b))
	}
	return "unknown"
}

func sessionType() string {
	st := os.Getenv("XDG_SESSION_TYPE")
	if st == "" {
		st = "unknown"
	}
	if wt := os.Getenv("WAYLAND_DISPLAY"); wt != "" {
		return st + ", wayland display " + wt
	}
	return st
}

func timeSince(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return time.Since(t).Round(time.Second).String()
}
