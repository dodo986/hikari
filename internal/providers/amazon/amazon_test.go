package amazon

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

const fakeNile = `#!/bin/sh
case "$1 $2" in
  "library sync")
    mkdir -p "$NILE_CONFIG_PATH"
    printf '[{"product":{"id":"amzn-tamago","title":"Tamago Quest","productDetail":{"details":{"developer":"Fake Dev","backgroundUrl2":"https://cdn.example.test/bg2.jpg","logoUrl":"https://cdn.example.test/logo.jpg"},"iconUrl":"https://cdn.example.test/icon.jpg"}}}]' > "$NILE_CONFIG_PATH/library.json"
    exit 0 ;;
  "install --info")
    printf '{"download_size":524288000,"disk_size":734003200,"version":"1.0"}'
    exit 0 ;;
  "install --base-path")
    echo "Progress: 10.0 "
    echo "Downloaded: 100.00 MiB"
    echo "Download	- 2.00 MiB"
    echo "Disk	- 1.00 MiB"
    echo "ETA: 00:04:10"
    echo "Progress: 100.0 "
    echo "Downloaded: 1000.00 MiB"
    mkdir -p "$NILE_CONFIG_PATH"
    printf '[{"id":"amzn-tamago","version":"1.0","path":"/games/TamagoQuest","size":1048576000}]' > "$NILE_CONFIG_PATH/installed.json"
    exit 0 ;;
  "uninstall "*)
    rm -f "$NILE_CONFIG_PATH/installed.json"
    exit 0 ;;
esac
case "$1" in
  auth)
    case " $* " in
      *" --login "*)
        printf '{"url":"https://example.test/amazon-login","code_verifier":"cv","serial":"sr","client_id":"cid"}'
        exit 0 ;;
      *" --logout "*)
        rm -f "$NILE_CONFIG_PATH/current_user.json"
        exit 0 ;;
    esac
    exit 1 ;;
  register)
    echo "[AUTH_MANAGER]: Info: Succesfully registered a device" >&2
    mkdir -p "$NILE_CONFIG_PATH"
    printf '{"user_id":"u-1","name":"FakeAmazonUser"}' > "$NILE_CONFIG_PATH/current_user.json"
    exit 0 ;;
esac
echo "unexpected call: $*" >&2
exit 1
`

func newTestProvider(t *testing.T) (*Provider, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "nile")
	if err := os.WriteFile(bin, []byte(fakeNile), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{NileBin: bin}, nil)
	run := engine.NewRunner(slog.New(slog.DiscardHandler))
	return New(eng, run, paths), paths
}

func TestListFromNileState(t *testing.T) {
	p, _ := newTestProvider(t)
	games, err := p.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 0 {
		t.Fatalf("empty nile state should list nothing, got %+v", games)
	}
}

func TestRefreshThenList(t *testing.T) {
	p, _ := newTestProvider(t)
	n, err := p.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("refreshed %d games, want 1", n)
	}
	games, _ := p.List(context.Background())
	if len(games) != 1 || games[0].Title != "Tamago Quest" {
		t.Fatalf("bad list after refresh: %+v", games)
	}
	g := games[0]
	if g.Platform != "Windows" || !g.CanRunOffline || g.Runner != library.RunnerNile {
		t.Fatalf("bad flags: %+v", g)
	}
	if g.Artwork.Cover != "https://cdn.example.test/bg2.jpg" || g.Artwork.Square != "https://cdn.example.test/icon.jpg" {
		t.Fatalf("bad artwork: %+v", g.Artwork)
	}
	if g.Installed {
		t.Error("not installed yet")
	}
}

func TestInstallInfo(t *testing.T) {
	p, _ := newTestProvider(t)
	if _, err := p.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	info, err := p.InstallInfo(context.Background(), "amzn-tamago", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Tamago Quest" || info.Version != "1.0" {
		t.Fatalf("bad info: %+v", info)
	}
	if info.DownloadSize != 524288000 || info.DiskSize != 734003200 {
		t.Fatalf("bad sizes: %+v", info)
	}
}

func TestInstallWithProgress(t *testing.T) {
	p, _ := newTestProvider(t)
	var frames []library.InstallProgress
	err := p.Install(context.Background(), "amzn-tamago", library.InstallOptions{Path: "/games"}, func(pr library.InstallProgress) {
		frames = append(frames, pr)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < 4 {
		t.Fatalf("too few frames: %+v", frames)
	}
	var sawPercent bool
	for _, f := range frames {
		if f.Percent != nil && *f.Percent == 100 {
			sawPercent = true
		}
	}
	if !sawPercent {
		t.Fatalf("no 100%% frame: %+v", frames)
	}
	games, _ := p.List(context.Background())
	if len(games) != 1 || !games[0].Installed || games[0].InstallPath != "/games/TamagoQuest" {
		t.Fatalf("install not reflected: %+v", games)
	}
}

func TestAuthFlowWithPendingDeviceData(t *testing.T) {
	p, _ := newTestProvider(t)
	st, _ := p.AuthStatus(context.Background())
	if st.LoggedIn {
		t.Fatal("should start logged out")
	}
	info, err := p.BeginLogin(context.Background())
	if err != nil || info.URL != "https://example.test/amazon-login" {
		t.Fatalf("begin: %+v %v", info, err)
	}
	p2, _ := newTestProvider(t)
	if _, err := p2.CompleteLogin(context.Background(), "x"); err == nil {
		t.Fatal("complete without begin should fail")
	}
	st, err = p.CompleteLogin(context.Background(), "authz-code")
	if err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn || st.User != "FakeAmazonUser" || st.UserID != "u-1" {
		t.Fatalf("login failed: %+v", st)
	}
	if err := p.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st, _ := p.AuthStatus(context.Background()); st.LoggedIn {
		t.Fatal("logout did not clear state")
	}
}

func TestMissingEngine(t *testing.T) {
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{NileBin: "/nonexistent/nile"}, nil)
	p := New(eng, engine.NewRunner(slog.New(slog.DiscardHandler)), paths)
	_, err := p.Refresh(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nile not found") {
		t.Fatalf("want missing-engine error, got %v", err)
	}
}
