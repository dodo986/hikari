package epic

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

const fakeLegendary = `#!/bin/sh
case "$1" in
  list)
    echo "[cli] INFO: Logged in as FakeUser"
    exit 0 ;;
  info)
    echo '{"game":{"title":"Sushi Racer","version":"1.0.4","launch_options":[{"name":"Default","parameters":""},{"name":"Windowed","parameters":"-windowed -nosound"}],"owned_dlc":[{"app_name":"dlc-1","title":"Sushi Racer OST"}]},"manifest":{"download_size":524288000,"disk_size":734003200}}'
    exit 0 ;;
  repair)
    echo "Verifying files..."
    echo "Downloaded: 1.00 MiB"
    exit 0 ;;
  sync-saves)
    echo "Syncing saves to /tmp/fake-saves"
    exit 0 ;;
  install)
    echo "Download size: 500.00 MiB"
    echo "Downloaded: 50.00 MiB"
    echo "Download	- 5.00 MiB"
    echo "Disk	- 4.00 MiB"
    echo "ETA: 00:01:30"
    echo "Downloaded: 250.00 MiB"
    mkdir -p "$LEGENDARY_CONFIG_PATH"
    printf '{"b1a2c3d4e5f6":{"install_path":"/games/SushiRacer","executable":"SushiRacer/Bin/Racer.exe","version":"1.0.4","platform":"Windows","install_size":12884901888}}' > "$LEGENDARY_CONFIG_PATH/installed.json"
    exit 0 ;;
  uninstall)
    rm -f "$LEGENDARY_CONFIG_PATH/installed.json"
    exit 0 ;;
  update)
    echo "Download size: 200.00 MiB"
    echo "Downloaded: 100.00 MiB"
    echo "ETA: 00:01:00"
    exit 0 ;;
  auth)
    case " $* " in
      *" --delete "*)
        rm -f "$LEGENDARY_CONFIG_PATH/user.json"
        exit 0 ;;
      *" --code "*)
        echo "ERROR: Logging in failed" >&2
        exit 1 ;;
    esac
    exit 0 ;;
esac
echo "unexpected call: $*" >&2
exit 1
`

const fakeLegendaryOK = `#!/bin/sh
case "$1" in
  auth)
    printf '{"account_id":"acc-123","displayName":"FakeUser"}' > "$LEGENDARY_CONFIG_PATH/user.json"
    exit 0 ;;
esac
exit 1
`

func newTestProvider(t *testing.T, script string) (*Provider, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "legendary")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{LegendaryBin: bin}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	return New(eng, engine.NewRunner(slog.New(slog.DiscardHandler)), paths), paths
}

func seedMetadata(t *testing.T, paths config.Paths, app string) {
	t.Helper()
	meta := `{
  "metadata": {
    "title": "Sushi Racer",
    "developer": "Fake Studio",
    "namespace": "ns-racer",
    "keyImages": [
      {"type": "DieselGameBox", "url": "https://cdn.example.test/cover.jpg"},
      {"type": "DieselGameBoxTall", "url": "https://cdn.example.test/square.jpg"},
      {"type": "DieselGameBoxLogo", "url": "https://cdn.example.test/logo.jpg"}
    ],
    "categories": [{"path": "games"}],
    "releaseInfo": [{"platform": ["Windows", "Mac"]}],
    "customAttributes": {
      "CanRunOffline": {"value": "true"},
      "CloudSaveFolder": {"value": "{InstallDir}"},
      "FolderName": {"value": "SushiRacer"}
    }
  }
}`
	dir := filepath.Join(paths.EngineEpic, "metadata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, app+".json"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListFromEngineState(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	seedMetadata(t, paths, "b1a2c3d4e5f6")
	inst := `{"b1a2c3d4e5f6":{"install_path":"/games/SushiRacer","executable":"SushiRacer/Bin/Racer.exe","version":"1.0.4","platform":"Windows","install_size":12884901888,"save_path":null}}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
		t.Fatal(err)
	}

	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("want 1 game, got %+v", games)
	}
	g := games[0]
	if g.Title != "Sushi Racer" || !g.Installed || g.Platform != "Windows" {
		t.Fatalf("bad game: %+v", g)
	}
	if g.Artwork.Cover != "https://cdn.example.test/cover.jpg" || g.Artwork.Logo != "https://cdn.example.test/logo.jpg" {
		t.Fatalf("bad artwork: %+v", g.Artwork)
	}
	if !g.CanRunOffline || !g.CloudSaves || g.Metadata["namespace"] != "ns-racer" {
		t.Fatalf("bad flags: %+v %v", g, g.Metadata)
	}
	if g.InstallSize != "12.00 GiB" {
		t.Fatalf("install size: want %q, got %q", "12.00 GiB", g.InstallSize)
	}
	if g.Version != "1.0.4" || g.InstallPath != "/games/SushiRacer" {
		t.Fatalf("install fields: %+v", g)
	}
}

func TestListFiltersUEAndMods(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	seedMetadata(t, paths, "ue-title")
	f := filepath.Join(paths.EngineEpic, "metadata", "ue-title.json")
	if err := os.WriteFile(f, []byte(strings.Replace(readFile(t, f), "ns-racer", "ue", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	seedMetadata(t, paths, "mod-title")
	f = filepath.Join(paths.EngineEpic, "metadata", "mod-title.json")
	if err := os.WriteFile(f, []byte(strings.Replace(readFile(t, f), `"path": "games"`, `"path": "mods"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	games, _ := p.List(context.Background())
	if len(games) != 0 {
		t.Fatalf("UE and mod titles must be filtered, got %+v", games)
	}
}

func TestInstalledWithoutMetadataStillListed(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	inst := `{"orphan":{"install_path":"/games/Orphan","platform":"Windows","version":"1.0"}}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
		t.Fatal(err)
	}
	games, _ := p.List(context.Background())
	if len(games) != 1 || !games[0].Installed || games[0].ID != "orphan" {
		t.Fatalf("orphan install missing: %+v", games)
	}
}

func TestInstallRunsEngineWithProgress(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	seedMetadata(t, paths, "b1a2c3d4e5f6")

	var frames []library.InstallProgress
	err := p.Install(context.Background(), "b1a2c3d4e5f6", library.InstallOptions{
		Path: "/games", Platform: "Windows", MaxWorkers: 4,
	}, func(pr library.InstallProgress) { frames = append(frames, pr) })
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < 3 {
		t.Fatalf("too few progress frames: %+v", frames)
	}
	last := frames[len(frames)-1]
	if last.State != "done" || last.Percent == nil || *last.Percent != 100 {
		t.Fatalf("no done frame: %+v", last)
	}
	games, _ := p.List(context.Background())
	if len(games) != 1 || !games[0].Installed {
		t.Fatalf("install not reflected: %+v", games)
	}
}

func TestUninstall(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	inst := `{"b1a2c3d4e5f6":{"install_path":"/games/SushiRacer","platform":"Windows"}}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.Uninstall(context.Background(), "b1a2c3d4e5f6", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.EngineEpic, "installed.json")); !os.IsNotExist(err) {
		t.Fatal("engine should have removed installed.json entry")
	}
}

func TestLaunchOptions(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	seedMetadata(t, paths, "b1a2c3d4e5f6g7h8i9j0")
	inst := `{"b1a2c3d4e5f6g7h8i9j0":{"install_path":"/games/SushiRacer","platform":"Windows"},
	          "dlc-1":{"install_path":"/games/DLC","platform":"Windows","executable":"DLC/dlc.exe"}}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
		t.Fatal(err)
	}
	g := library.Game{Store: library.StoreEpic, ID: "b1a2c3d4e5f6g7h8i9j0", Platform: "Windows"}
	opts, err := p.LaunchOptions(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != 3 {
		t.Fatalf("want 3 options, got %+v", opts)
	}
	if opts[1].Name != "Windowed" || len(opts[1].Params) != 2 || opts[1].Params[1] != "-nosound" {
		t.Fatalf("params not split: %+v", opts[1])
	}
	if opts[2].Type != "dlc" || opts[2].DLCApp != "dlc-1" {
		t.Fatalf("dlc option wrong: %+v", opts[2])
	}
}

func TestRepairAndSaves(t *testing.T) {
	p, _ := newTestProvider(t, fakeLegendary)
	var frames []library.InstallProgress
	if err := p.Repair(context.Background(), "b1a2c3d4e5f6g7h8i9j0", library.InstallOptions{}, func(pr library.InstallProgress) {
		frames = append(frames, pr)
	}); err != nil {
		t.Fatal(err)
	}
	if len(frames) < 2 || frames[len(frames)-1].State != "done" {
		t.Fatalf("repair frames: %+v", frames)
	}
	if _, err := p.SyncSaves(context.Background(), "b1a2c3d4e5f6g7h8i9j0", library.SaveOptions{}); err == nil {
		t.Fatal("sync without save path must error")
	}
	out, err := p.SyncSaves(context.Background(), "b1a2c3d4e5f6g7h8i9j0", library.SaveOptions{
		Direction: "upload", SavePath: "/tmp/fake-saves",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Syncing saves") {
		t.Fatalf("output: %q", out)
	}
}

func TestInstallInfo(t *testing.T) {
	p, _ := newTestProvider(t, fakeLegendary)
	info, err := p.InstallInfo(context.Background(), "b1a2c3d4e5f6g7h8i9j0", "Windows")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Sushi Racer" || info.Version != "1.0.4" {
		t.Fatalf("bad info: %+v", info)
	}
	if info.DownloadSize != 524288000 || info.DiskSize != 734003200 {
		t.Fatalf("bad sizes: %+v", info)
	}
	if len(info.DLCs) != 1 || info.DLCs[0].ID != "dlc-1" || info.DLCs[0].Title != "Sushi Racer OST" {
		t.Fatalf("bad dlcs: %+v", info.DLCs)
	}
}

func TestUpdateAvailableFromAssets(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendary)
	seedMetadata(t, paths, "b1a2c3d4e5f6g7h8i9j0")
	inst := `{"b1a2c3d4e5f6g7h8i9j0":{"install_path":"/games/SushiRacer","platform":"Windows","version":"1.0.0"}}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
		t.Fatal(err)
	}
	assets := `{"Windows":[{"app_name":"b1a2c3d4e5f6g7h8i9j0","build_version":"1.0.4"},{"app_name":"other","build_version":"9.9"}]}`
	if err := os.WriteFile(filepath.Join(paths.EngineEpic, "assets.json"), []byte(assets), 0o600); err != nil {
		t.Fatal(err)
	}
	games, _ := p.List(context.Background())
	for _, g := range games {
		if g.ID == "b1a2c3d4e5f6g7h8i9j0" {
			if !g.UpdateAvailable {
				t.Fatal("update should be flagged: asset 1.0.4 vs installed 1.0.0")
			}
		}
	}
	assets = `{"Windows":[{"app_name":"b1a2c3d4e5f6g7h8i9j0","build_version":"1.0.0"}]}`
	os.WriteFile(filepath.Join(paths.EngineEpic, "assets.json"), []byte(assets), 0o600)
	games, _ = p.List(context.Background())
	for _, g := range games {
		if g.ID == "b1a2c3d4e5f6g7h8i9j0" && g.UpdateAvailable {
			t.Fatal("same version must not flag an update")
		}
	}
}

func TestUpdateRunsEngine(t *testing.T) {
	p, _ := newTestProvider(t, fakeLegendary)
	var frames []library.InstallProgress
	err := p.Update(context.Background(), "b1a2c3d4e5f6g7h8i9j0", library.InstallOptions{MaxWorkers: 2}, func(pr library.InstallProgress) {
		frames = append(frames, pr)
	})
	if err != nil {
		t.Fatal(err)
	}
	if frames[0].State != "updating" || frames[len(frames)-1].State != "done" {
		t.Fatalf("update frames wrong: %+v", frames)
	}
	var saw bool
	for _, f := range frames {
		if f.Bytes == "100.00MB" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("update progress not parsed: %+v", frames)
	}
}

func TestAuthFlow(t *testing.T) {
	p, paths := newTestProvider(t, fakeLegendaryOK)

	st, err := p.AuthStatus(context.Background())
	if err != nil || st.LoggedIn {
		t.Fatalf("logged out status wrong: %+v %v", st, err)
	}
	info, err := p.BeginLogin(context.Background())
	if err != nil || !strings.Contains(info.URL, "legendary.gl/epiclogin") {
		t.Fatalf("begin login: %+v %v", info, err)
	}
	st, err = p.CompleteLogin(context.Background(), "fake-code")
	if err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn || st.User != "FakeUser" || st.UserID != "acc-123" {
		t.Fatalf("login failed: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(paths.EngineEpic, "user.json")); err != nil {
		t.Fatal("engine should have written user.json")
	}
}

func TestAuthFailureIsActionable(t *testing.T) {
	p, _ := newTestProvider(t, fakeLegendary)
	_, err := p.CompleteLogin(context.Background(), "bad-code")
	if err == nil || !strings.Contains(err.Error(), "epic login failed") {
		t.Fatalf("want actionable error, got %v", err)
	}
}

func TestMissingEngineSurfacesRequirement(t *testing.T) {
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{LegendaryBin: "/nonexistent/legendary"}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	p := New(eng, engine.NewRunner(slog.New(slog.DiscardHandler)), paths)
	_, err := p.Refresh(context.Background())
	if err == nil || !strings.Contains(err.Error(), "legendary not found") {
		t.Fatalf("want missing-engine error, got %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
