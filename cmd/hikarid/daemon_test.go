package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hikari/internal/api"
	"hikari/internal/art"
	"hikari/internal/library"
)

const fakeLegendary = `#!/bin/sh
case "$1" in
  install)
    echo "Download size: 100.00 MiB"
    echo "Downloaded: 1.00 MiB"
    sleep 300
    exit 0 ;;
  launch)
    sleep 300
    exit 0 ;;
  update)
    echo "Downloaded: 50.00 MiB"
    exit 0 ;;
  repair)
    echo "Verifying..."
    exit 0 ;;
esac
echo "unexpected: $*" >&2
exit 1
`

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func startDaemon(t *testing.T) (*daemon, *api.Client) {
	t.Helper()
	cfgRoot := t.TempDir()
	dataRoot := t.TempDir()
	runtime := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("HIKARI_HEROIC_CONFIG", filepath.Join(cfgRoot, "heroic-absent"))

	bin := filepath.Join(t.TempDir(), "legendary")
	if err := os.WriteFile(bin, []byte(fakeLegendary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfgRoot, "hikari"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := "[engines]\nlegendary_bin = \"" + bin + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgRoot, "hikari", "settings.toml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	engDir := filepath.Join(cfgRoot, "hikari", "engines", "legendary")
	if err := os.MkdirAll(engDir, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := `{"nativegame":{"install_path":"/games/Native","platform":"Linux","version":"1.0"}}`
	if err := os.WriteFile(filepath.Join(engDir, "installed.json"), []byte(installed), 0o600); err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(engDir, "metadata")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"metadata":{"title":"Native Game","keyImages":[{"type":"DieselGameBox","url":"https://cdn.example.test/nativegame/cover.jpg"}]}}`
	if err := os.WriteFile(filepath.Join(metaDir, "nativegame.json"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := newDaemon(discardLogger(), false)
	if err != nil {
		t.Fatal(err)
	}
	d.watchInterval = 40 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	srv := api.NewServer(discardLogger())
	d.register(srv, ctx)
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx, filepath.Join(runtime, "hikari.sock")) }()
	t.Cleanup(func() { cancel(); <-done })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client, err := api.Dial(filepath.Join(runtime, "hikari.sock"))
		if err == nil {
			return d, client
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon socket never came up")
	return nil, nil
}

func awaitProgress(t *testing.T, client *api.Client, state string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		params, err := client.AwaitEvent("install.progress", deadline.Sub(time.Now()))
		if err != nil {
			break
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) == nil && p.State == state {
			return
		}
	}
	t.Fatalf("no install.progress with state %q", state)
}

func TestArtworkLocalFillsAsync(t *testing.T) {
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer imgSrv.Close()

	d, client := startDaemon(t)
	d.art = art.NewWithGetter(filepath.Join(t.TempDir(), "art"), &loopbackGetter{base: imgSrv.URL})

	if err := client.Subscribe("library.changed"); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Games []library.Game `json:"games"`
	}
	if err := client.Call("library.list", map[string]any{"store": "epic"}, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Games) == 0 {
		t.Fatal("fixture game missing")
	}
	if res.Games[0].ArtworkLocal.Cover != "" {
		t.Fatal("first list should have no local artwork yet")
	}
	params, err := client.AwaitEvent("library.changed", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var ev struct {
		Store string `json:"store"`
	}
	if json.Unmarshal(params, &ev) != nil || ev.Store != "epic" {
		t.Fatalf("bad library.changed: %s", params)
	}
	if err := client.Call("library.list", map[string]any{"store": "epic"}, &res); err != nil {
		t.Fatal(err)
	}
	if res.Games[0].ArtworkLocal.Cover == "" || !strings.HasSuffix(res.Games[0].ArtworkLocal.Cover, ".jpg") {
		t.Fatalf("artwork_local not filled: %+v", res.Games[0].ArtworkLocal)
	}
}

type loopbackGetter struct{ base string }

func (g *loopbackGetter) Get(url string) (*http.Response, error) {
	return http.Get(g.base + "/cover.jpg")
}

func TestUpdateEndToEnd(t *testing.T) {
	_, client := startDaemon(t)
	engDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "hikari", "engines", "legendary")
	inst := filepath.Join(engDir, "installed.json")
	if err := os.Chtimes(inst, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		_ = err
	}
	if err := client.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := client.Call("game.update", map[string]any{"store": "epic", "id": "nativegame"}, &res); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		params, err := client.AwaitEvent("install.progress", deadline.Sub(time.Now()))
		if err != nil {
			break
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) == nil {
			if p.State == "done" {
				return
			}
			if p.State == "error" {
				t.Fatal("update errored")
			}
		}
	}
	t.Fatal("no done event for update")
}

func TestFavoriteRoundTrip(t *testing.T) {
	_, client := startDaemon(t)
	var res map[string]any
	if err := client.Call("game.favorite", map[string]any{"store": "epic", "id": "nativegame"}, &res); err != nil {
		t.Fatal(err)
	}
	if res["favorite"] != true {
		t.Fatalf("toggle on: %+v", res)
	}
	var list struct {
		Games []struct {
			ID       string `json:"id"`
			Favorite bool   `json:"favorite"`
		} `json:"games"`
	}
	if err := client.Call("library.list", nil, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range list.Games {
		if g.ID == "nativegame" && g.Favorite {
			found = true
		}
	}
	if !found {
		t.Fatal("favorite not listed")
	}
	var one struct {
		ID       string `json:"id"`
		Favorite bool   `json:"favorite"`
	}
	if err := client.Call("library.get", map[string]any{"store": "epic", "id": "nativegame"}, &one); err != nil {
		t.Fatal(err)
	}
	if !one.Favorite {
		t.Fatal("library.get dropped the favorite flag")
	}
	if err := client.Call("game.favorite", map[string]any{
		"store": "epic", "id": "nativegame", "favorite": false,
	}, &res); err != nil {
		t.Fatal(err)
	}
	if res["favorite"] != false {
		t.Fatalf("explicit off: %+v", res)
	}
}

func TestLocalGamesRoundTrip(t *testing.T) {
	_, client := startDaemon(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var g library.Game
	if err := client.Call("game.add_local", map[string]any{
		"title": "Mine", "executable": exe, "platform": "Linux", "args": []string{"-x"},
	}, &g); err != nil {
		t.Fatal(err)
	}
	if g.Store != "local" || g.Runner != "local" || !g.Installed {
		t.Fatalf("bad local game: %+v", g)
	}
	var list struct {
		Games []library.Game `json:"games"`
	}
	if err := client.Call("library.list", map[string]any{"store": "local"}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Games) != 1 || list.Games[0].ID != g.ID {
		t.Fatalf("local list: %+v", list.Games)
	}
	var plan struct {
		Plan struct {
			Argv []string `json:"argv"`
		} `json:"plan"`
	}
	if err := client.Call("game.launch", map[string]any{
		"store": "local", "id": g.ID, "dry_run": true,
	}, &plan); err != nil {
		t.Fatal(err)
	}
	if strings.Join(plan.Plan.Argv, " ") != exe+" -x" {
		t.Fatalf("local plan: %q", plan.Plan.Argv)
	}
	if err := client.Call("game.remove_local", map[string]any{"id": g.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("library.list", map[string]any{"store": "local"}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Games) != 0 {
		t.Fatalf("remove failed: %+v", list.Games)
	}
}

func TestLaunchOptionInDryRun(t *testing.T) {
	_, client := startDaemon(t)
	var res struct {
		Plan struct {
			Argv []string `json:"argv"`
		} `json:"plan"`
	}
	if err := client.Call("game.launch", map[string]any{
		"store": "epic", "id": "nativegame", "dry_run": true,
		"option": map[string]any{"name": "Windowed", "type": "basic", "params": []string{"-windowed", "-nosound"}},
	}, &res); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Plan.Argv, " ")
	if !strings.Contains(joined, "-windowed -nosound") {
		t.Fatalf("option params missing from plan: %s", joined)
	}
}

func TestRepairEndToEnd(t *testing.T) {
	_, client := startDaemon(t)
	if err := client.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := client.Call("game.repair", map[string]any{"store": "epic", "id": "nativegame"}, &res); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		params, err := client.AwaitEvent("install.progress", deadline.Sub(time.Now()))
		if err != nil {
			break
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal(params, &p) == nil && p.State == "done" {
			return
		}
	}
	t.Fatal("no done event for repair")
}

func TestSavesNeedPath(t *testing.T) {
	_, client := startDaemon(t)
	err := client.Call("game.saves.sync", map[string]any{
		"store": "epic", "id": "nativegame", "direction": "upload",
	}, nil)
	if err == nil {
		t.Fatal("sync without save_path should fail")
	}
	if !strings.Contains(err.Error(), "save path") {
		t.Fatalf("error should mention the save path: %v", err)
	}
}

func TestLibraryChangedPushesOnStateFileChange(t *testing.T) {
	_, client := startDaemon(t)
	if err := client.Subscribe("library.changed"); err != nil {
		t.Fatal(err)
	}

	installed := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "hikari", "engines", "legendary", "installed.json")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(installed, future, future); err != nil {
		t.Fatal(err)
	}
	params, err := client.AwaitEvent("library.changed", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var ev struct {
		Store string `json:"store"`
	}
	if json.Unmarshal(params, &ev) != nil || ev.Store != "epic" {
		t.Fatalf("bad library.changed: %s", params)
	}
}

func TestInstallCancelEndToEnd(t *testing.T) {
	_, client := startDaemon(t)
	if err := client.Subscribe("install.progress"); err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := client.Call("game.install", map[string]any{
		"store": "epic", "id": "whatever", "path": t.TempDir(),
	}, &res); err != nil {
		t.Fatal(err)
	}
	awaitProgress(t, client, "downloading")

	if err := client.Call("game.install.cancel", map[string]any{
		"store": "epic", "id": "whatever",
	}, &res); err != nil {
		t.Fatal(err)
	}
	awaitProgress(t, client, "canceled")

	err := client.Call("game.install.cancel", map[string]any{"store": "epic", "id": "whatever"}, &res)
	var re *api.Error
	if apiErr, ok := err.(*api.Error); !ok || apiErr.Code != api.CodeBusy {
		_ = re
		t.Fatalf("want -32002 for idle cancel, got %v", err)
	}
}

func TestLaunchDryRunCarriesUmuID(t *testing.T) {
	_, client := startDaemon(t)

	dataDir := os.Getenv("XDG_DATA_HOME")
	if err := os.WriteFile(filepath.Join(dataDir, "hikari", "umu_ids.json"),
		[]byte(`{"ids":{"egs_nativegame":"umu-777"},"misses":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Plan struct {
			Env map[string]string `json:"env"`
		} `json:"plan"`
	}
	if err := client.Call("game.launch", map[string]any{
		"store": "epic", "id": "nativegame", "dry_run": true,
	}, &res); err != nil {
		t.Fatal(err)
	}
	if res.Plan.Env["GAMEID"] != "umu-777" {
		t.Fatalf("GAMEID = %q, want umu-777 from cache", res.Plan.Env["GAMEID"])
	}
}

func TestLaunchRunningAndStopEndToEnd(t *testing.T) {
	_, client := startDaemon(t)

	var res map[string]any
	if err := client.Call("game.launch", map[string]any{
		"store": "epic", "id": "nativegame",
	}, &res); err != nil {
		t.Fatal(err)
	}
	if res["state"] != "launching" {
		t.Fatalf("launch reply = %v, want launching", res)
	}

	var running struct {
		Games []struct {
			Store string `json:"store"`
			ID    string `json:"id"`
			State string `json:"state"`
			Pid   int    `json:"pid"`
		} `json:"games"`
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Call("game.running", nil, &running); err != nil {
			t.Fatal(err)
		}
		if len(running.Games) == 1 && running.Games[0].Pid != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(running.Games) != 1 {
		t.Fatalf("game never appeared in game.running: %+v", running)
	}
	g := running.Games[0]
	if g.Store != "epic" || g.ID != "nativegame" || g.State != "playing" || g.Pid == 0 {
		t.Fatalf("running entry wrong: %+v", g)
	}

	if err := client.Call("game.stop", map[string]any{"store": "epic", "id": "nativegame"}, &res); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("game.running", nil, &running); err != nil {
		t.Fatal(err)
	}
	if len(running.Games) != 0 {
		t.Fatalf("stopped game still listed: %+v", running)
	}
}
