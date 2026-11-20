package gog

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

const fakeGogdl = `#!/bin/sh
if [ "$1" = "--auth-config-path" ]; then
  AUTH="$2"; shift 2
else
  echo "missing --auth-config-path" >&2; exit 1
fi
case "$1" in
  auth)
    case " $* " in
      *" --code "*)
        printf '{"access_token":"x","refresh_token":"y","user_id":"u-7"}' > "$AUTH"
        printf '{"access_token":"<redacted>","user_id":"u-7"}'
        exit 0 ;;
    esac
    exit 0 ;;
  info)
    echo '{"game":{"app_name":"1207658919","title":"Wasabi Wars","version":"2.0"},"download":{"buildId":"b-1","versionName":"2.1","languages":["en-US"],"folder_name":"WasabiWars","size":{"*":{"download_size":1073741824,"disk_size":1610612736}},"dlcs":[{"title":"Wasabi OST","id":"dlc-9","size":{"*":{"download_size":104857600}}}]}}'
    exit 0 ;;
  download)
    echo "Progress: 25.0 "
    echo "Downloaded: 250.00 MiB"
    echo "Download	- 3.00 MiB"
    echo "Disk	- 2.00 MiB"
    echo "ETA: 00:08:20"
    echo "Progress: 99.9 "
    echo "Downloaded: 999.00 MiB"
    exit 0 ;;
esac
echo "unexpected call: $*" >&2
exit 1
`

func newTestProvider(t *testing.T) (*Provider, config.Paths) {
	t.Helper()
	return newProviderWithGogdl(t, fakeGogdl)
}

// newProviderWithGogdl is newTestProvider with a caller-supplied gogdl script.
func newProviderWithGogdl(t *testing.T, script string) (*Provider, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "gogdl")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{GogdlBin: bin}, nil)
	run := engine.NewRunner(slog.New(slog.DiscardHandler))
	return New(eng, run, paths), paths
}

// infoOnlyGogdl is a fake gogdl that prints infoJSON for `info`. When argsFile
// is non-empty it appends every invocation's gogdl arguments (after the auth
// header) to that file.
func infoOnlyGogdl(infoJSON, argsFile string) string {
	record := ""
	if argsFile != "" {
		record = `echo "$*" >> "` + argsFile + `"` + "\n"
	}
	return `#!/bin/sh
if [ "$1" = "--auth-config-path" ]; then
  shift 2
fi
` + record + `if [ "$1" = "info" ]; then
  printf '%s' '` + infoJSON + `'
  exit 0
fi
echo "unexpected call: $*" >&2
exit 1
`
}

func TestAuthFlow(t *testing.T) {
	p, paths := newTestProvider(t)
	st, _ := p.AuthStatus(context.Background())
	if st.LoggedIn {
		t.Fatal("should start logged out")
	}
	info, err := p.BeginLogin(context.Background())
	if err != nil || !strings.HasPrefix(info.URL, "https://auth.gog.com/auth?") {
		t.Fatalf("begin: %+v %v", info, err)
	}
	st, err = p.CompleteLogin(context.Background(), "gog-code")
	if err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn || st.UserID != "u-7" {
		t.Fatalf("login failed: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(paths.EngineGOG, "auth.json")); err != nil {
		t.Fatal("gogdl should have written auth.json")
	}
	if err := p.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st, _ := p.AuthStatus(context.Background()); st.LoggedIn {
		t.Fatal("logout should remove the credentials")
	}
}

func TestInstallPopulatesCacheWithProgress(t *testing.T) {
	p, paths := newTestProvider(t)
	var frames []library.InstallProgress
	err := p.Install(context.Background(), "1207658919", library.InstallOptions{
		Path: "/games/WasabiWars", Language: "en-US",
	}, func(pr library.InstallProgress) { frames = append(frames, pr) })
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < 4 {
		t.Fatalf("too few frames: %+v", frames)
	}
	if frames[0].State != "downloading" || frames[len(frames)-1].State != "done" {
		t.Fatalf("frame states wrong: %+v", frames)
	}

	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("cache not written: %+v", games)
	}
	g := games[0]
	if g.Title != "Wasabi Wars" || !g.Installed || g.InstallPath != "/games/WasabiWars" || g.Platform != "windows" {
		t.Fatalf("bad cache entry: %+v", g)
	}
	if _, err := os.Stat(filepath.Join(paths.DataDir, "libraries", "gog.json")); err != nil {
		t.Fatal("cache file missing")
	}
}

func TestUninstallRemovesFilesAndCache(t *testing.T) {
	p, _ := newTestProvider(t)
	if err := p.Install(context.Background(), "1207658919", library.InstallOptions{Path: filepath.Join(t.TempDir(), "WasabiWars")}, nil); err != nil {
		t.Fatal(err)
	}
	games, _ := p.List(context.Background())
	installDir := games[0].InstallPath
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "WasabiWars.exe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := p.Uninstall(context.Background(), "1207658919", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatal("install dir should be gone")
	}
	games, _ = p.List(context.Background())
	if len(games) != 1 || games[0].Installed {
		t.Fatalf("cache should keep the entry as uninstalled: %+v", games)
	}
}

func TestLaunchOptionsFromInfoFile(t *testing.T) {
	p, _ := newTestProvider(t)
	gameDir := t.TempDir()
	info := `{
  "name": "Wasabi Wars",
  "playTasks": [
    {"type": "FileTask", "name": "Play", "path": "game.exe"},
    {"type": "FileTask", "name": "Safe mode", "path": "game.exe", "arguments": "-safe"},
    {"type": "FileTask", "name": "Manual", "path": "manual.pdf", "category": "document"},
    {"type": "URLTask", "name": "Website", "link": "https://example.test"}
  ]
}`
	if err := os.WriteFile(filepath.Join(gameDir, "goggame-1207658919.info"), []byte(info), 0o600); err != nil {
		t.Fatal(err)
	}
	g := library.Game{Store: library.StoreGOG, ID: "1207658919", InstallPath: gameDir}
	opts, err := p.LaunchOptions(context.Background(), g)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != 2 {
		t.Fatalf("want the two FileTasks, got %+v", opts)
	}
	if opts[1].Name != "Safe mode" || len(opts[1].Params) != 2 || opts[1].Params[1] != "1" {
		t.Fatalf("prefer-task index wrong: %+v", opts[1])
	}
	one := t.TempDir()
	os.WriteFile(filepath.Join(one, "goggame-1.info"), []byte(`{"playTasks":[{"type":"FileTask","name":"Play"}]}`), 0o600)
	opts, err = p.LaunchOptions(context.Background(), library.Game{Store: library.StoreGOG, ID: "1", InstallPath: one})
	if err != nil || len(opts) != 0 {
		t.Fatalf("single task should yield no options: %+v %v", opts, err)
	}
}

func TestInstallInfo(t *testing.T) {
	p, _ := newTestProvider(t)
	info, err := p.InstallInfo(context.Background(), "1207658919", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Wasabi Wars" || info.Version != "2.1" {
		t.Fatalf("bad info: %+v", info)
	}
	if info.DownloadSize != 1073741824 || info.DiskSize != 1610612736 {
		t.Fatalf("bad sizes: %+v", info)
	}
	if len(info.DLCs) != 1 || info.DLCs[0].ID != "dlc-9" || info.DLCs[0].DownloadSize != 104857600 {
		t.Fatalf("bad dlcs: %+v", info.DLCs)
	}
}

func TestUninstallUnknownGame(t *testing.T) {
	p, _ := newTestProvider(t)
	err := p.Uninstall(context.Background(), "unknown", false)
	if err == nil || !strings.Contains(err.Error(), "not known") {
		t.Fatalf("want unknown-game error, got %v", err)
	}
}

// flatGogdlInfo is what `gogdl info` really prints for a v1/v2 manager: a flat
// download-manager dict with no {game,download} wrapper and no title.
const flatGogdlInfo = `{"size":{"*":{"download_size":1073741824,"disk_size":1610612736},"en-US":{"download_size":1048576,"disk_size":2097152}},"download_size":1024,"disk_size":2048,"dlcs":[{"id":"dlc-9","title":"Wasabi OST","size":{"*":{"download_size":104857600}}},{"id":"dlc-10","title":"Wasabi Art"}],"buildId":"b-1","languages":["en-US"],"folder_name":"WasabiWars","versionName":"2.1"}`

func TestInstallInfoParsesFlatGogdlOutput(t *testing.T) {
	p, _ := newProviderWithGogdl(t, infoOnlyGogdl(flatGogdlInfo, ""))
	p.upsertCache(entry{ID: "1207658919", Title: "Wasabi Wars", Language: "en-US"})

	info, err := p.InstallInfo(context.Background(), "1207658919", "Mac")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Wasabi Wars" {
		t.Fatalf("title must come from the cache, got %q", info.Title)
	}
	if info.Version != "2.1" {
		t.Fatalf("version: %+v", info)
	}
	// size["*"] + size["en-US"] + the flat top-level fields.
	if want := int64(1073741824 + 1048576 + 1024); info.DownloadSize != want {
		t.Fatalf("download size = %d, want %d", info.DownloadSize, want)
	}
	if want := int64(1610612736 + 2097152 + 2048); info.DiskSize != want {
		t.Fatalf("disk size = %d, want %d", info.DiskSize, want)
	}
	if len(info.DLCs) != 2 || info.DLCs[0].ID != "dlc-9" || info.DLCs[0].DownloadSize != 104857600 || info.DLCs[1].Title != "Wasabi Art" {
		t.Fatalf("dlcs: %+v", info.DLCs)
	}
}

func TestInstallInfoParsesFlatLinuxSizes(t *testing.T) {
	const linux = `{"download_size":524288000,"disk_size":734003200,"buildId":"b-2","version":"1.4"}`
	p, _ := newProviderWithGogdl(t, infoOnlyGogdl(linux, ""))

	info, err := p.InstallInfo(context.Background(), "7", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.DownloadSize != 524288000 || info.DiskSize != 734003200 {
		t.Fatalf("flat sizes: %+v", info)
	}
	if info.Version != "1.4" {
		t.Fatalf("version must fall back to the flat version field: %+v", info)
	}
}

func TestPlatformNormalisedForGogdl(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	p, _ := newProviderWithGogdl(t, infoOnlyGogdl(flatGogdlInfo, argsFile))
	for _, platform := range []string{"Mac", ""} {
		if _, err := p.InstallInfo(context.Background(), "1", platform); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("recorded %d gogdl calls: %q", len(lines), lines)
	}
	if !strings.HasSuffix(lines[0], "--os osx") {
		t.Fatalf("Mac must reach gogdl as osx: %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], "--os windows") {
		t.Fatalf("an empty platform must reach gogdl as windows: %q", lines[1])
	}
}

func TestInstallKeepsRefreshedArtwork(t *testing.T) {
	p, _ := newTestProvider(t)
	if err := p.writeCache([]entry{{
		ID: "1207658919", Title: "Wasabi Wars", Developer: "Fake Studio",
		Cover: "cover.jpg", Square: "square.jpg", Icon: "icon.jpg",
		LibraryPlatform: "Linux",
	}}); err != nil {
		t.Fatal(err)
	}

	err := p.Install(context.Background(), "1207658919", library.InstallOptions{
		Path: "/games/WasabiWars", Language: "en-US",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("cache: %+v", games)
	}
	g := games[0]
	if g.Developer != "Fake Studio" || g.Artwork.Cover != "cover.jpg" || g.Artwork.Square != "square.jpg" || g.Artwork.Icon != "icon.jpg" {
		t.Fatalf("install wiped the refreshed metadata: %+v", g)
	}
	if !g.Installed || g.InstallPath != "/games/WasabiWars" || g.Version != "2.1" {
		t.Fatalf("install state not updated: %+v", g)
	}
}

func TestRefreshKeepsInstallThatRacedIt(t *testing.T) {
	release := make(chan struct{})
	metaStarted := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()

	const meta = `{"type":"game","external_id":"1207658919","title":{"*":"Wasabi Wars"},"game":{"visible_in_library":true,"title":{"*":"Wasabi Wars"},"developers":[{"name":"Fake Studio"}],"logo":{"url_format":"https://images.gog.com/logo.{ext}"}},"supported_operating_systems":[{"slug":"windows"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/users/"):
			w.Write([]byte(`{"items":[{"platform_id":"gog","external_id":"1207658919","owned":true,"certificate":"cert-1"}]}`))
		case strings.Contains(r.URL.Path, "/external_releases/"):
			close(metaStarted)
			<-release
			w.Write([]byte(meta))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p, paths := newTestProvider(t)
	if err := os.WriteFile(filepath.Join(paths.EngineGOG, "auth.json"),
		[]byte(`{"access_token":"tok","refresh_token":"r","user_id":"u-7"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p.galaxy = NewGalaxyClient()
	p.galaxy.libBase = srv.URL
	p.galaxy.dbBase = srv.URL

	done := make(chan error, 1)
	go func() {
		_, err := p.Refresh(context.Background())
		done <- err
	}()
	<-metaStarted

	// The refresh is parked in GameMeta; a whole install completes meanwhile.
	err := p.Install(context.Background(), "1207658919", library.InstallOptions{Path: "/games/WasabiWars"}, nil)
	unblock()
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got library.Game
	for _, g := range games {
		if g.ID == "1207658919" {
			got = g
		}
	}
	if !got.Installed || got.InstallPath != "/games/WasabiWars" {
		t.Fatalf("the refresh overwrote the completed install: %+v", got)
	}
}
