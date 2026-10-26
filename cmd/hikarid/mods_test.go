package main

import (
	"encoding/json"
	"errors"
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
