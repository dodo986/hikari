package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hikari/internal/api"
	"hikari/internal/library"
	"hikari/internal/mods"
)

func awaitModsDone(t *testing.T, client *api.Client) map[string]any {
	t.Helper()
	params, err := client.AwaitEvent("mods.done", 10*time.Second)
	if err != nil {
		t.Fatalf("no mods.done event: %v", err)
	}
	var event map[string]any
	if err := json.Unmarshal(params, &event); err != nil {
		t.Fatalf("mods.done is not an object: %v", err)
	}
	return event
}

func TestModsStatusAndGames(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()

	var status mods.Status
	if err := client.Call("mods.status", nil, &status); err != nil {
		t.Fatalf("mods.status: %v", err)
	}
	if !status.Available {
		t.Fatalf("mods.status reported %+v, want available", status)
	}

	var list struct {
		Games []modsGameView `json:"games"`
	}
	if err := client.Call("mods.games", nil, &list); err != nil {
		t.Fatalf("mods.games: %v", err)
	}
	if len(list.Games) != 3 {
		t.Fatalf("mods.games returned %d games, want 3", len(list.Games))
	}
	first := list.Games[0]
	if first.ID != "skyrim_se" || !first.Configured || len(first.Profiles) != 2 {
		t.Fatalf("first game = %+v, want skyrim_se with two profiles", first)
	}
	if first.Store != "" || first.GameID != "" {
		t.Fatalf("first game carries store %q / game_id %q, want no library match", first.Store, first.GameID)
	}
}

func TestModsDeployPublishesProgressAndDone(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	if err := client.Subscribe("mods.progress", "mods.done", "mods.log"); err != nil {
		t.Fatal(err)
	}

	var start struct {
		Job string `json:"job"`
	}
	if err := client.Call("mods.deploy", map[string]any{"game": "skyrim_se", "profile": "default"}, &start); err != nil {
		t.Fatalf("mods.deploy: %v", err)
	}
	if start.Job == "" {
		t.Fatal("mods.deploy returned no job id")
	}

	params, err := client.AwaitEvent("mods.progress", 10*time.Second)
	if err != nil {
		t.Fatalf("no mods.progress event: %v", err)
	}
	var progress struct {
		Job     string `json:"job"`
		Phase   string `json:"phase"`
		Current int    `json:"current"`
		Total   int    `json:"total"`
	}
	if err := json.Unmarshal(params, &progress); err != nil {
		t.Fatalf("mods.progress is not an object: %v", err)
	}
	if progress.Job != start.Job || progress.Phase != "deploying" {
		t.Fatalf("mods.progress = %+v, want job %s deploying", progress, start.Job)
	}
	if progress.Total == 0 || progress.Current == 0 {
		t.Fatalf("mods.progress = %+v, want a measured tick", progress)
	}

	done := awaitModsDone(t, client)
	if done["ok"] != true {
		t.Fatalf("mods.done = %v, want ok", done)
	}

	var list struct {
		Games []modsGameView `json:"games"`
	}
	if err := client.Call("mods.games", nil, &list); err != nil {
		t.Fatal(err)
	}
	if !list.Games[0].DeployActive || list.Games[0].DeployedProfile != "default" {
		t.Fatalf("after deploy the game is %+v, want deploy_active on default", list.Games[0])
	}
}

func TestModsDeployFailuresReachDone(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	if err := client.Subscribe("mods.progress", "mods.done", "mods.log"); err != nil {
		t.Fatal(err)
	}
	var start struct {
		Job string `json:"job"`
	}
	if err := client.Call("mods.deploy", map[string]any{"game": "no_such_game"}, &start); err != nil {
		t.Fatalf("mods.deploy: %v", err)
	}
	done := awaitModsDone(t, client)
	if done["ok"] != false {
		t.Fatalf("mods.done = %v, want ok false", done)
	}
	message, _ := done["error"].(string)
	if message == "" {
		t.Fatalf("mods.done = %v, want an error message", done)
	}
}

func TestModsJobsAreSerialised(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	if err := client.Subscribe("mods.progress", "mods.done", "mods.log"); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("mods.deploy", map[string]any{"game": "skyrim_se"}, nil); err != nil {
		t.Fatalf("first mods.deploy: %v", err)
	}
	err := client.Call("mods.deploy", map[string]any{"game": "cyberpunk_2077"}, nil)
	if err == nil {
		t.Fatal("second mods.deploy was accepted, want a busy error")
	}
	var rpcErr *api.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != api.CodeBusy {
		t.Fatalf("second mods.deploy error = %v, want code %d", err, api.CodeBusy)
	}
	awaitModsDone(t, client)
}

func TestModsCancelStopsTheRunningJob(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	var done map[string]any
	client.OnEvent = func(method string, params json.RawMessage) {
		if method == "mods.done" {
			_ = json.Unmarshal(params, &done)
		}
	}
	if err := client.Subscribe("mods.progress", "mods.done", "mods.log"); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("mods.deploy", map[string]any{"game": "skyrim_se"}, nil); err != nil {
		t.Fatalf("mods.deploy: %v", err)
	}
	var cancel struct {
		Canceled bool `json:"canceled"`
	}
	if err := client.Call("mods.cancel", nil, &cancel); err != nil {
		t.Fatalf("mods.cancel: %v", err)
	}
	if !cancel.Canceled {
		t.Fatal("mods.cancel replied without canceling")
	}
	for done == nil {
		if _, err := client.AwaitEvent("mods.done", 10*time.Second); err != nil {
			t.Fatalf("no mods.done after cancel: %v", err)
		}
	}
	if ok, _ := done["ok"].(bool); ok {
		t.Fatalf("cancelled job reported ok: %v", done)
	}
	if done["error"] != "the job was cancelled" {
		t.Fatalf("error = %v, want the cancellation message", done["error"])
	}
	if err := client.Call("mods.cancel", nil, nil); err == nil {
		t.Fatal("mods.cancel with no job running was accepted")
	}
}

func TestMatchLibraryGame(t *testing.T) {
	known := []library.Game{
		{Store: library.StoreSteam, ID: "489830", Title: "Skyrim Special Edition", Installed: true},
		{Store: library.StoreGOG, ID: "1234", Title: "Cyberpunk 2077", Installed: true, InstallPath: "/games/Cyberpunk 2077/"},
		{Store: library.StoreEpic, ID: "epic1", Title: "Not Installed", Installed: false, InstallPath: "/games/Not Installed"},
	}
	cases := []struct {
		name   string
		target mods.Game
		want   string
		found  bool
	}{
		{"steam app id", mods.Game{SteamAppID: "489830", Name: "whatever"}, "steam/489830", true},
		{"install path", mods.Game{Path: "/games/Cyberpunk 2077", Name: "whatever"}, "gog/1234", true},
		{"title", mods.Game{Name: "skyrim special edition"}, "steam/489830", true},
		{"installed only", mods.Game{Name: "Not Installed"}, "", false},
		{"unknown", mods.Game{Name: "Half-Life"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			game, ok := matchLibraryGame(known, tc.target)
			if ok != tc.found {
				t.Fatalf("matchLibraryGame(%+v) found = %v, want %v", tc.target, ok, tc.found)
			}
			if ok && game.Key() != tc.want {
				t.Fatalf("matchLibraryGame(%+v) = %s, want %s", tc.target, game.Key(), tc.want)
			}
		})
	}
}

func TestModsSettingsApplyWithoutARestart(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.New(mods.Options{Root: t.TempDir()})

	var before mods.Status
	if err := client.Call("mods.status", nil, &before); err != nil {
		t.Fatalf("mods.status: %v", err)
	}
	if before.Available {
		t.Fatalf("status before setup = %+v, want unavailable", before)
	}

	source := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "cli.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	settings := "[mods]\nsource_dir = \"" + source + "\"\n"
	if err := os.WriteFile(d.paths.Settings, []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	var after mods.Status
	if err := client.Call("mods.status", nil, &after); err != nil {
		t.Fatalf("mods.status after the edit: %v", err)
	}
	if after.Source != source {
		t.Fatalf("status source = %q after editing settings.toml, want %q", after.Source, source)
	}
}

func TestModsBrowsingEndpoints(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	if _, err := d.mods.Login(context.Background(), mods.Sink{}); err != nil {
		t.Fatalf("demo login: %v", err)
	}

	var page mods.Browse
	if err := client.Call("mods.browse", map[string]any{"game": "skyrim_se"}, &page); err != nil {
		t.Fatalf("mods.browse: %v", err)
	}
	if len(page.Results) == 0 || page.Domain == "" {
		t.Fatalf("mods.browse returned %+v", page)
	}

	var files struct {
		Mod   mods.Mod       `json:"mod"`
		Files []mods.ModFile `json:"files"`
	}
	if err := client.Call("mods.files", map[string]any{"game": "skyrim_se", "mod_id": page.Results[0].ModID}, &files); err != nil {
		t.Fatalf("mods.files: %v", err)
	}
	if files.Mod.ModID != page.Results[0].ModID || len(files.Files) == 0 {
		t.Fatalf("mods.files returned %+v", files)
	}
	// The browser-download probe the free-account install flow polls.
	var found struct {
		Found string `json:"found"`
	}
	if err := client.Call("mods.downloads", map[string]any{"game": "skyrim_se", "mod_id": files.Mod.ModID, "file_id": files.Files[0].FileID}, &found); err != nil {
		t.Fatalf("mods.downloads: %v", err)
	}
	if found.Found != "" {
		t.Fatalf("mods.downloads invented a download: %+v", found)
	}

	// Newest build first, and each row says when it was uploaded: the sheet
	// shows both without asking for anything else.
	for i, file := range files.Files {
		if file.Uploaded == "" {
			t.Fatalf("file %d has no upload date: %+v", i, file)
		}
		if i > 0 && files.Files[i-1].Uploaded < file.Uploaded {
			t.Fatalf("files are not newest first: %s then %s",
				files.Files[i-1].Uploaded, file.Uploaded)
		}
	}
	if page.Results[0].Category == "" || page.Results[0].Updated == "" {
		t.Fatalf("browse results lost their category or update date: %+v", page.Results[0])
	}

	var list mods.Installed
	if err := client.Call("mods.installed", map[string]any{"game": "skyrim_se"}, &list); err != nil {
		t.Fatalf("mods.installed: %v", err)
	}
	if len(list.Mods) == 0 {
		t.Fatalf("mods.installed returned %+v", list)
	}

	if err := client.Call("mods.browse", map[string]any{}, nil); err == nil {
		t.Fatal("mods.browse accepted params without a game")
	}
	if err := client.Call("mods.files", map[string]any{"game": "skyrim_se"}, nil); err == nil {
		t.Fatal("mods.files accepted no mod id")
	}
}

func TestModsLoginAndLogout(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()
	if err := client.Subscribe("mods.done", "mods.auth_url", "mods.log"); err != nil {
		t.Fatal(err)
	}

	var account mods.Account
	if err := client.Call("mods.account", nil, &account); err != nil {
		t.Fatalf("mods.account: %v", err)
	}
	if account.Stored {
		t.Fatalf("the demo starts signed out, got %+v", account)
	}

	var started struct {
		Job string `json:"job"`
	}
	if err := client.Call("mods.login", nil, &started); err != nil {
		t.Fatalf("mods.login: %v", err)
	}
	if started.Job == "" {
		t.Fatal("mods.login returned no job")
	}
	if done := awaitModsDone(t, client); done["ok"] != true {
		t.Fatalf("mods.done = %+v", done)
	}
	if err := client.Call("mods.account", nil, &account); err != nil {
		t.Fatalf("mods.account after login: %v", err)
	}
	if !account.Stored || account.Mode != "oauth" {
		t.Fatalf("mods.account returned %+v", account)
	}

	if err := client.Call("mods.logout", nil, nil); err != nil {
		t.Fatalf("mods.logout: %v", err)
	}
	if err := client.Call("mods.account", nil, &account); err != nil {
		t.Fatalf("mods.account after logout: %v", err)
	}
	if account.Stored {
		t.Fatalf("mods.account still signed in after logout: %+v", account)
	}
}

// The classic mod-manager loop: toggle a row, uninstall a mod, and ask what the
// profile is still missing.
func TestModsToggleRemoveAndRequirements(t *testing.T) {
	d, client := startDaemon(t)
	d.mods = mods.NewFake()

	var installed mods.Installed
	if err := client.Call("mods.installed", map[string]any{"game": "skyrim_se"}, &installed); err != nil {
		t.Fatalf("mods.installed: %v", err)
	}
	if len(installed.Mods) == 0 {
		t.Fatal("the demo profile is empty")
	}
	name := installed.Mods[0].Name

	var toggled struct {
		Enabled bool `json:"enabled"`
	}
	if err := client.Call("mods.toggle", map[string]any{"game": "skyrim_se", "name": name}, &toggled); err != nil {
		t.Fatalf("mods.toggle: %v", err)
	}
	if toggled.Enabled {
		t.Fatalf("%q was already enabled, the toggle did nothing", name)
	}

	if err := client.Subscribe("mods.done"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	var job struct {
		Job string `json:"job"`
	}
	if err := client.Call("mods.remove", map[string]any{"game": "skyrim_se", "name": name}, &job); err != nil {
		t.Fatalf("mods.remove: %v", err)
	}
	if done := awaitModsDone(t, client); done["ok"] != true {
		t.Fatalf("remove failed: %+v", done)
	}
	var after mods.Installed
	if err := client.Call("mods.installed", map[string]any{"game": "skyrim_se"}, &after); err != nil {
		t.Fatalf("mods.installed: %v", err)
	}
	for _, mod := range after.Mods {
		if mod.Name == name {
			t.Fatalf("%q is still in the profile after removal", name)
		}
	}

	var missing struct {
		Missing []mods.Missing `json:"missing"`
	}
	if err := client.Call("mods.missing", map[string]any{"game": "skyrim_se"}, &missing); err != nil {
		t.Fatalf("mods.missing: %v", err)
	}
	if len(missing.Missing) == 0 || len(missing.Missing[0].Needs) == 0 {
		t.Fatalf("mods.missing returned %+v", missing)
	}
}
