package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"hikari/internal/api"
	"hikari/internal/library"
)

func startDemoDaemon(t *testing.T) (*daemon, *api.Client) {
	t.Helper()
	cfgRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HIKARI_HEROIC_CONFIG", cfgRoot+"/heroic-absent")

	d, err := newDaemon(discardLogger(), true)
	if err != nil {
		t.Fatal(err)
	}
	d.watchInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv := api.NewServer(discardLogger())
	d.register(srv, ctx)
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx, runtime+"/hikari.sock") }()
	t.Cleanup(func() { cancel(); <-done })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client, err := api.Dial(runtime + "/hikari.sock")
		if err == nil {
			return d, client
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("demo daemon socket never came up")
	return nil, nil
}

func TestDemoLibraryShape(t *testing.T) {
	_, client := startDemoDaemon(t)
	var res struct {
		Games []library.Game `json:"games"`
	}
	if err := client.Call("library.list", nil, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Games) != 12 {
		t.Fatalf("want 12 demo games, got %d", len(res.Games))
	}
	byStore := map[string]int{}
	installed := 0
	for _, g := range res.Games {
		byStore[g.Store]++
		if g.Installed {
			installed++
		}
		if g.Source != library.SourceDemo {
			t.Fatalf("demo game with wrong source: %+v", g)
		}

		if g.ArtworkLocal.Cover == "" {
			t.Fatalf("demo cover not seeded locally: %s", g.Title)
		}
		if _, err := os.Stat(g.ArtworkLocal.Cover); err != nil {
			t.Fatalf("seeded cover missing on disk: %v", err)
		}
	}
	for store, want := range map[string]int{"epic": 4, "gog": 3, "amazon": 2, "steam": 3} {
		if byStore[store] != want {
			t.Fatalf("store %s has %d games, want %d", store, byStore[store], want)
		}
	}
	if installed == 0 || installed == 12 {
		t.Fatalf("demo should mix installed and not-installed: %d installed", installed)
	}

	var notInstalled, isInstalled *library.Game
	for i := range res.Games {
		if res.Games[i].Installed && isInstalled == nil {
			isInstalled = &res.Games[i]
		}
		if !res.Games[i].Installed && notInstalled == nil {
			notInstalled = &res.Games[i]
		}
	}
	var info library.InstallInfo
	if err := client.Call("game.install_info", map[string]any{
		"store": notInstalled.Store, "id": notInstalled.ID,
	}, &info); err != nil {
		t.Fatal(err)
	}
	if info.DownloadSize == 0 || info.Title == "" {
		t.Fatalf("install_info empty: %+v", info)
	}
}

func TestDemoInstallProgressAndCancel(t *testing.T) {
	_, client := startDemoDaemon(t)
	if err := client.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := client.Call("game.install", map[string]any{
		"store": "epic", "id": "demo-epic-3", "path": "/opt/demo-games/demo-epic-3",
	}, &res); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AwaitEvent("install.progress", 3*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("game.install.cancel", map[string]any{
		"store": "epic", "id": "demo-epic-3",
	}, &res); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		params, err := client.AwaitEvent("install.progress", deadline.Sub(time.Now()))
		if err != nil {
			break
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) == nil && p.State == "canceled" {

			var lib struct {
				Games []library.Game `json:"games"`
			}
			if err := client.Call("library.list", map[string]any{"store": "epic"}, &lib); err != nil {
				t.Fatal(err)
			}
			for _, g := range lib.Games {
				if g.ID == "demo-epic-3" && g.Installed {
					t.Fatal("canceled install left the game installed")
				}
			}
			return
		}
	}
	t.Fatal("no canceled event")
}

func TestDemoLaunchRunningStop(t *testing.T) {
	_, client := startDemoDaemon(t)
	if err := client.Subscribe("game.state"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := client.Call("game.launch", map[string]any{
		"store": "steam", "id": "demo-steam-1",
	}, &res); err != nil {
		t.Fatal(err)
	}
	var running struct {
		Games []struct {
			Store string `json:"store"`
			ID    string `json:"id"`
			State string `json:"state"`
			Pid   int    `json:"pid"`
		} `json:"games"`
	}
	if err := client.Call("game.running", nil, &running); err != nil {
		t.Fatal(err)
	}
	if len(running.Games) != 1 || running.Games[0].ID != "demo-steam-1" || running.Games[0].Pid == 0 {
		t.Fatalf("demo launch not tracked: %+v", running)
	}
	err := client.Call("game.launch", map[string]any{"store": "steam", "id": "demo-steam-1", "dry_run": true}, &res)
	if re, ok := err.(*api.Error); !ok || re.Code != api.CodeUnsupported {
		t.Fatalf("demo dry-run should be -32001, got %v", err)
	}
	if err := client.Call("game.stop", map[string]any{"store": "steam", "id": "demo-steam-1"}, &res); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("game.running", nil, &running); err != nil {
		t.Fatal(err)
	}
	if len(running.Games) != 0 {
		t.Fatalf("demo game still running after stop: %+v", running)
	}
}

func TestDemoAuth(t *testing.T) {
	_, client := startDemoDaemon(t)
	var res struct {
		Stores map[string]library.AuthStatus `json:"stores"`
	}
	if err := client.Call("auth.status", nil, &res); err != nil {
		t.Fatal(err)
	}
	if !res.Stores["epic"].LoggedIn || res.Stores["epic"].User != "DemoPlayer" {
		t.Fatalf("epic should be demo-logged-in: %+v", res.Stores["epic"])
	}
	if err := client.Call("auth.logout", map[string]any{"store": "gog"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("auth.status", nil, &res); err != nil {
		t.Fatal(err)
	}
	if res.Stores["gog"].LoggedIn {
		t.Fatal("logout did not clear gog")
	}
}
