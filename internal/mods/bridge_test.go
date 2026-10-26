package mods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/engine"
)

// newTestBridge points a Bridge at a temp Amethyst tree and replaces the
// embedded script with a sh program, so the protocol is exercised end to end
// without Python or a real install.
func newTestBridge(t *testing.T, script string) *Bridge {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "amethyst", "src")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "cli.py"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	bridge := New(Options{Root: root, Source: source, Python: "/bin/sh"})
	if bridge.blocked != "" {
		t.Fatalf("bridge blocked in test setup: %s", bridge.blocked)
	}
	if err := os.WriteFile(bridge.scriptPath(), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bridge
}

func TestBridgeGamesParsesProtocol(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "src=$HIKARI_AMETHYST_SRC"
echo '{"t":"log","m":"Loading handlers"}'
echo 'a diagnostic that is not JSON'
echo '{"t":"result","data":{"games":[{"id":"skyrim_se","name":"Skyrim Special Edition","steam_app_id":"489830","configured":true,"profiles":["default"],"deploy_active":true,"deployed_profile":"default"}]}}'
`)
	games, err := bridge.Games(context.Background())
	if err != nil {
		t.Fatalf("Games returned %v, want no error", err)
	}
	if len(games) != 1 {
		t.Fatalf("Games returned %d games, want 1", len(games))
	}
	got := games[0]
	if got.ID != "skyrim_se" || got.SteamAppID != "489830" || !got.DeployActive || got.DeployedProfile != "default" {
		t.Fatalf("Games returned %+v, want skyrim_se/489830/deployed default", got)
	}
	if len(got.Profiles) != 1 || got.Profiles[0] != "default" {
		t.Fatalf("Games returned profiles %v, want [default]", got.Profiles)
	}
}

func TestBridgeForwardsLogAndProgress(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "argv=$@"
echo '{"t":"log","m":"Hardlink preflight: shared device"}'
echo '{"t":"progress","cur":4,"total":9,"m":"Transferring"}'
echo '{"t":"result","data":{"deployed":true}}'
`)
	var logs []string
	var progress []Progress
	err := bridge.Deploy(context.Background(), "skyrim_se", "default", Sink{
		Log:      func(line string) { logs = append(logs, line) },
		Progress: func(p Progress) { progress = append(progress, p) },
	})
	if err != nil {
		t.Fatalf("Deploy returned %v, want no error", err)
	}
	if len(logs) != 2 || logs[0] != "argv=deploy --game skyrim_se --profile default" || logs[1] != "Hardlink preflight: shared device" {
		t.Fatalf("Deploy logged %q, want the argv line then the backend line", logs)
	}
	if len(progress) != 1 || progress[0].Current != 4 || progress[0].Total != 9 || progress[0].Message != "Transferring" {
		t.Fatalf("Deploy reported %+v, want one 4/9 Transferring tick", progress)
	}
}

func TestBridgeReportsBackendError(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo '{"t":"error","m":"no Nexus API key is stored","kind":"config"}'
exit 1
`)
	_, err := bridge.Install(context.Background(), InstallRequest{Game: "skyrim_se", URL: "nxm://skyrimspecialedition/mods/1/files/2"}, Sink{})
	if err == nil {
		t.Fatal("Install returned no error, want the backend's failure")
	}
	if KindOf(err) != KindConfig {
		t.Fatalf("Install error kind = %q (%v), want %q", KindOf(err), err, KindConfig)
	}
	if !strings.Contains(err.Error(), "no Nexus API key") {
		t.Fatalf("Install error = %q, want the backend's message", err)
	}
}

func TestBridgeReportsSilentFailure(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "Traceback: boom" >&2
exit 1
`)
	err := bridge.Restore(context.Background(), "skyrim_se", Sink{})
	if err == nil {
		t.Fatal("Restore returned no error, want a failure")
	}
	if KindOf(err) != KindFailure {
		t.Fatalf("Restore error kind = %q (%v), want %q", KindOf(err), err, KindFailure)
	}
	if !strings.Contains(err.Error(), "Traceback: boom") {
		t.Fatalf("Restore error = %q, want the last stderr line", err)
	}
}

func TestBridgeInstallReturnsModName(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo '{"t":"result","data":{"mod":"Hikari Test Mod-1.0","game":"Hikari Test Game"}}'
`)
	name, err := bridge.Install(context.Background(), InstallRequest{
		Game: "hikari_test", Profile: "default", Archive: "/tmp/mod.zip",
	}, Sink{})
	if err != nil {
		t.Fatalf("Install returned %v, want no error", err)
	}
	if name != "Hikari Test Mod-1.0" {
		t.Fatalf("Install returned %q, want the installed mod name", name)
	}
}

func TestBridgeInstallNeedsArchiveOrURL(t *testing.T) {
	bridge := newTestBridge(t, "#!/bin/sh\nexit 0\n")
	_, err := bridge.Install(context.Background(), InstallRequest{Game: "skyrim_se"}, Sink{})
	if KindOf(err) != KindUsage {
		t.Fatalf("Install with no source = %v (kind %q), want a usage error", err, KindOf(err))
	}
}

func TestBridgeBlockedWithoutAmethyst(t *testing.T) {
	bridge := New(Options{Root: t.TempDir(), Source: filepath.Join(t.TempDir(), "missing")})
	status := bridge.Status(context.Background())
	if status.Available {
		t.Fatal("Status reported available without an Amethyst tree")
	}
	if !strings.Contains(status.Reason, "Amethyst") {
		t.Fatalf("Status reason = %q, want it to name Amethyst", status.Reason)
	}
	_, err := bridge.Games(context.Background())
	if KindOf(err) != KindUnavailable {
		t.Fatalf("Games error = %v (kind %q), want unavailable", err, KindOf(err))
	}
}

func TestBridgeMaterialisesScript(t *testing.T) {
	bridge := newTestBridge(t, "#!/bin/sh\n")
	for _, path := range []string{bridge.scriptPath(), bridge.requirementsPath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("bridge did not write %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", path)
		}
	}
}

func TestBridgeHonoursCancellation(t *testing.T) {
	bridge := newTestBridge(t, "#!/bin/sh\nsleep 30\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := bridge.Deploy(ctx, "skyrim_se", "default", Sink{})
	if err == nil {
		t.Fatal("Deploy returned no error, want the cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Deploy error = %v, want context.Canceled", err)
	}
}

func TestParseEvent(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
		kind string
	}{
		{"result", `{"t":"result","data":{"mod":"x"}}`, true, "result"},
		{"log", `{"t":"log","m":"hi"}`, true, "log"},
		{"plain", `Extracting 2 files`, false, ""},
		{"json without t", `{"cur":1}`, false, ""},
		{"broken", `{"t":`, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseEvent(tc.line)
			if ok != tc.want {
				t.Fatalf("parseEvent(%q) ok = %v, want %v", tc.line, ok, tc.want)
			}
			if ok && got.Type != tc.kind {
				t.Fatalf("parseEvent(%q) type = %q, want %q", tc.line, got.Type, tc.kind)
			}
		})
	}
}

func TestLastLine(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"trailing blank", "one\ntwo\n\n", "two"},
		{"empty", "", "no output"},
		{"single", "boom", "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastLine(tc.in); got != tc.want {
				t.Fatalf("lastLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBridgeRunnerIsOptional(t *testing.T) {
	bridge := New(Options{Root: t.TempDir(), Source: "/nonexistent", Runner: engine.NewRunner(nil)})
	if bridge.runner == nil {
		t.Fatal("New did not keep the supplied runner")
	}
}

func TestAsSourceDirAcceptsTheCheckoutRoot(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "cli.py"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := asSourceDir(root); got != src {
		t.Fatalf("asSourceDir(%q) = %q, want %q", root, got, src)
	}
	if got := asSourceDir(src); got != src {
		t.Fatalf("asSourceDir(%q) = %q, want it unchanged", src, got)
	}
	if got := asSourceDir(filepath.Join(root, "nothing")); got != filepath.Join(root, "nothing") {
		t.Fatalf("asSourceDir kept neither form: %q", got)
	}
}

func TestDiscoverSourceFindsACheckoutUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HIKARI_MODS_SOURCE", "")
	src := filepath.Join(home, "games", "Amethyst-Mod-Manager", "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "cli.py"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := discoverSource(); got != src {
		t.Fatalf("discoverSource() = %q, want %q", got, src)
	}
}

func TestSetupWithoutASourceIsAConfigError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HIKARI_MODS_SOURCE", "")
	bridge := New(Options{Root: t.TempDir(), Python: "/bin/sh"})
	err := bridge.Setup(context.Background(), Sink{})
	var bridgeErr *Error
	if !errors.As(err, &bridgeErr) || bridgeErr.Kind != KindConfig {
		t.Fatalf("Setup error = %v, want a config error", err)
	}
	if !strings.Contains(bridgeErr.Message, "source_dir") {
		t.Fatalf("Setup error %q does not point at source_dir", bridgeErr.Message)
	}
}

func TestNexusKeyTravelsInTheEnvironment(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "key=$HIKARI_NEXUS_KEY"
echo "argv=$*"
echo '{"t":"result","data":{"stored":true}}'
`)
	stored, err := bridge.Nexus(context.Background(), "set", "  secret-key  ")
	if err != nil {
		t.Fatalf("Nexus(set) returned %v", err)
	}
	if !stored {
		t.Fatal("Nexus(set) reported no stored key")
	}
	if _, err := bridge.Nexus(context.Background(), "set", " "); err == nil {
		t.Fatal("Nexus(set) accepted an empty key")
	}
}

func TestBrowseSendsQuerySortAndPage(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "argv=$*"
echo '{"t":"result","data":{"game":"Skyrim Special Edition","domain":"skyrimspecialedition","page":2,"more":true,"results":[{"mod_id":12604,"name":"SkyUI","downloads":41000000,"endorsements":480000,"picture_url":"https://example.test/a.png"}]}}'
`)
	page, err := bridge.Browse(context.Background(), "skyrim_se", "ui", "endorsements", 2)
	if err != nil {
		t.Fatalf("Browse returned %v", err)
	}
	if page.More != true || page.Page != 2 || len(page.Results) != 1 {
		t.Fatalf("Browse returned %+v", page)
	}
	if got := page.Results[0]; got.ModID != 12604 || got.Endorsements != 480000 || got.PictureURL == "" {
		t.Fatalf("result not parsed: %+v", got)
	}
}

func TestBrowseRefusesWithoutAKey(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo '{"t":"error","kind":"config","m":"no Nexus API key is stored"}'
`)
	if _, err := bridge.Browse(context.Background(), "skyrim_se", "", "", 0); err == nil {
		t.Fatal("Browse without a key returned no error")
	} else if KindOf(err) != KindConfig {
		t.Fatalf("Browse error kind = %q, want config", KindOf(err))
	}
}

func TestFilesNeedsAModID(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo '{"t":"result","data":{"mod":{"mod_id":0},"files":[]}}'
`)
	if _, _, err := bridge.Files(context.Background(), "skyrim_se", 0); err == nil {
		t.Fatal("Files accepted mod id 0")
	}
}

func TestInstallFromBrowsePassesBothIDs(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "argv=$*"
echo '{"t":"result","data":{"mod":"SkyUI","archive":"/tmp/skyui.7z"}}'
`)
	name, err := bridge.Install(context.Background(), InstallRequest{
		Game: "skyrim_se", ModID: 12604, FileID: 1,
	}, Sink{})
	if err != nil {
		t.Fatalf("Install returned %v", err)
	}
	if name != "SkyUI" {
		t.Fatalf("Install returned %q", name)
	}
	if _, err := bridge.Install(context.Background(), InstallRequest{Game: "skyrim_se"}, Sink{}); err == nil {
		t.Fatal("Install without a source returned no error")
	}
}

func TestInstalledReadsTheModlist(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "argv=$*"
echo '{"t":"result","data":{"game":"Cyberpunk 2077","profile":"default","mods":[{"name":"CET","enabled":true},{"name":"RED4ext","enabled":false}]}}'
`)
	list, err := bridge.Installed(context.Background(), "cyberpunk_2077", "default")
	if err != nil {
		t.Fatalf("Installed returned %v", err)
	}
	if len(list.Mods) != 2 || !list.Mods[0].Enabled || list.Mods[1].Enabled {
		t.Fatalf("modlist not parsed: %+v", list)
	}
}

func TestFakeBackendAnswersBrowsing(t *testing.T) {
	fake := NewFake()
	if _, err := fake.Login(context.Background(), Sink{}); err != nil {
		t.Fatalf("fake Login returned %v", err)
	}
	page, err := fake.Browse(context.Background(), "skyrim_se", "", "downloads", 0)
	if err != nil {
		t.Fatalf("fake Browse returned %v", err)
	}
	if len(page.Results) == 0 {
		t.Fatal("the demo catalogue is empty")
	}
	mod, files, err := fake.Files(context.Background(), "skyrim_se", page.Results[0].ModID)
	if err != nil || mod.ModID == 0 || len(files) == 0 {
		t.Fatalf("fake Files returned %+v %+v %v", mod, files, err)
	}
	list, err := fake.Installed(context.Background(), "skyrim_se", "default")
	if err != nil || len(list.Mods) == 0 {
		t.Fatalf("fake Installed returned %+v %v", list, err)
	}
}

func TestLoginStreamsTheAuthorisationPage(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo '{"t":"url","u":"https://users.nexusmods.com/oauth/authorize?client_id=amethyst"}'
echo '{"t":"log","m":"waiting for you to authorise Hikari on the Nexus page"}'
echo '{"t":"result","data":{"stored":true,"mode":"oauth","user":"someone","premium":false}}'
`)
	var links []string
	account, err := bridge.Login(context.Background(), Sink{AuthURL: func(url string) {
		links = append(links, url)
	}})
	if err != nil {
		t.Fatalf("Login returned %v", err)
	}
	if len(links) != 1 || !strings.Contains(links[0], "users.nexusmods.com") {
		t.Fatalf("authorisation links = %v", links)
	}
	if !account.Stored || account.Mode != "oauth" || account.User != "someone" {
		t.Fatalf("Login returned %+v", account)
	}
}

func TestLogoutClearsTheLogin(t *testing.T) {
	bridge := newTestBridge(t, `#!/bin/sh
echo "argv=$*"
echo '{"t":"result","data":{"stored":false}}'
`)
	if err := bridge.Logout(context.Background()); err != nil {
		t.Fatalf("Logout returned %v", err)
	}
}
