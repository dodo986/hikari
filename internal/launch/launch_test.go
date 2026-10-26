package launch

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/library"
)

func testManager(t *testing.T) (*engine.Manager, config.Paths) {
	t.Helper()
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	eng := engine.NewManager(config.EnginesSettings{}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	return eng, paths
}

func withFakeBins(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}

func epicGame() library.Game {
	return library.Game{
		Store: library.StoreEpic, ID: "b1a2c3d4e5f6g7h8i9j0",
		Title: "Sushi Racer", Platform: "Windows", Installed: true,
		InstallPath: "/games/SushiRacer", Runner: library.RunnerLegendary,
		Source: library.SourceHeroicImport,
	}
}

func protonSettings() config.GameSettings {
	s := FactorySettings(config.DefaultsSettings{})
	s.WineVersion = config.WineVersion{
		Bin:  "/usr/share/steam/compatibilitytools.d/proton-fake/proton",
		Name: "proton-fake", Type: "proton",
	}
	s.WinePrefix = "/pfx/sushi"
	return s
}

func TestBuildEpicProtonViaUmu(t *testing.T) {
	binDir := withFakeBins(t, "umu-run", "legendary", "gogdl", "nile")
	eng, paths := testManager(t)
	plan, err := Build(epicGame(), protonSettings(), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WineType != WineProtonUmu {
		t.Fatalf("wine type = %s, want umu-proton", plan.WineType)
	}
	if filepath.Base(plan.Argv[0]) != "legendary" {
		t.Fatalf("engine argv[0] = %q, want resolved legendary", plan.Argv[0])
	}
	wantTail := []string{"launch", "b1a2c3d4e5f6g7h8i9j0", "--no-wine", "--wrapper", filepath.Join(binDir, "umu-run")}
	if !reflect.DeepEqual(plan.Argv[1:], wantTail) {
		t.Fatalf("argv tail = %q, want %q", plan.Argv[1:], wantTail)
	}
	for _, kv := range [][2]string{
		{"GAMEID", "umu-0"},
		{"STORE", "egs"},
		{"WINEPREFIX", "/pfx/sushi"},
		{"STEAM_COMPAT_DATA_PATH", "/pfx/sushi"},
		{"PROTONPATH", "/usr/share/steam/compatibilitytools.d/proton-fake"},
		{"STEAM_COMPAT_APP_ID", "0"},
		{"SteamGameId", "hikari-b1a2c3d4e5f6g7h8i9j0"},
		{"STEAM_COMPAT_INSTALL_PATH", "/games/SushiRacer"},
		{"WINEDLLOVERRIDES", ""},
	} {
		if kv[1] == "" {
			if _, ok := plan.Env[kv[0]]; ok && kv[0] == "WINEDLLOVERRIDES" {
				t.Errorf("%s should not be set for proton", kv[0])
			}
			continue
		}
		if plan.Env[kv[0]] != kv[1] {
			t.Errorf("env %s = %q, want %q", kv[0], plan.Env[kv[0]], kv[1])
		}
	}
}

func TestBuildEpicPlainWine(t *testing.T) {
	withFakeBins(t, "wine", "legendary")
	eng, paths := testManager(t)
	s := FactorySettings(config.DefaultsSettings{})
	s.WinePrefix = "/pfx/sushi"
	s.ShowMangoHud = true
	s.UseGameMode = true
	plan, err := Build(epicGame(), s, paths, eng, Options{Args: []string{"-windowed"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WineType != WinePlain {
		t.Fatalf("wine type = %s, want wine", plan.WineType)
	}
	if plan.Argv[4] != "--wine" || !strings.HasSuffix(plan.Argv[5], "/wine") {
		t.Fatalf("plain wine should pass --wine <resolved bin>: %q", plan.Argv)
	}
	wantTail := []string{"--wrapper", "mangohud --dlsym gamemoderun"}
	if !reflect.DeepEqual(plan.Argv[6:], wantTail) {
		t.Fatalf("argv tail = %q, want %q", plan.Argv[6:], wantTail)
	}
	if plan.Env["WINEPREFIX"] != "/pfx/sushi" || plan.Env["WINEDLLOVERRIDES"] != "winemenubuilder.exe=d" {
		t.Fatalf("wine env wrong: %v", plan.Env)
	}
	if plan.Env["WINEESYNC"] != "1" || plan.Env["WINEFSYNC"] != "1" {
		t.Fatalf("esync/fsync defaults lost: %v", plan.Env)
	}
}

func TestBuildGogAndNile(t *testing.T) {
	binDir := withFakeBins(t, "umu-run", "legendary", "gogdl", "nile")
	umu := filepath.Join(binDir, "umu-run")
	eng, paths := testManager(t)

	gogGame := library.Game{
		Store: library.StoreGOG, ID: "1207658919", Title: "Wasabi Wars",
		Platform: "windows", Installed: true, InstallPath: "/games/WasabiWars",
		Runner: library.RunnerGogdl, Source: library.SourceHeroicImport,
	}
	plan, err := Build(gogGame, protonSettings(), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(plan.Argv[0]) != "gogdl" {
		t.Fatalf("gog engine argv[0] = %q", plan.Argv[0])
	}
	want := []string{"--auth-config-path", filepath.Join(paths.EngineGOG, "auth.json"),
		"launch", "/games/WasabiWars", "1207658919",
		"--no-wine", "--wrapper", umu, "--platform", "windows",
	}
	if !reflect.DeepEqual(plan.Argv[1:], want) {
		t.Fatalf("gog argv tail = %q, want %q", plan.Argv[1:], want)
	}
	if plan.Env["STORE"] != "gog" {
		t.Fatalf("gog STORE env = %q", plan.Env["STORE"])
	}

	nileGame := library.Game{
		Store: library.StoreAmazon, ID: "amzn-tamago", Title: "Tamago Quest",
		Platform: "Windows", Installed: true, InstallPath: "/games/TamagoQuest",
		Runner: library.RunnerNile, Source: library.SourceHeroicImport,
	}
	plan, err = Build(nileGame, protonSettings(), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(plan.Argv[0]) != "nile" {
		t.Fatalf("nile engine argv[0] = %q", plan.Argv[0])
	}
	want = []string{"launch", "--no-wine", "--wrapper", umu,
		"--wine-prefix", "/pfx/sushi", "amzn-tamago",
	}
	if !reflect.DeepEqual(plan.Argv[1:], want) {
		t.Fatalf("nile argv tail = %q, want %q", plan.Argv[1:], want)
	}
}

func TestBuildLocalNativeGame(t *testing.T) {
	eng, paths := testManager(t)
	g := library.Game{
		Store: "local", ID: "local-1", Title: "Mine", Runner: library.RunnerLocal,
		Platform: "Linux", Installed: true, Executable: "/games/mine/run.sh",
	}
	settings := FactorySettings(config.DefaultsSettings{})
	settings.ShowMangoHud = true
	plan, err := Build(g, settings, paths, eng, Options{Args: []string{"-x"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mangohud", "--dlsym", "/games/mine/run.sh", "-x"}
	if !reflect.DeepEqual(plan.Argv, want) {
		t.Fatalf("argv = %q, want %q", plan.Argv, want)
	}
	if plan.WineType != WineNative || plan.Engine != "local" {
		t.Fatalf("plan: %+v", plan)
	}
}

func TestBuildLocalWindowsThroughWine(t *testing.T) {
	withFakeBins(t, "wine")
	eng, paths := testManager(t)
	g := library.Game{
		Store: "local", ID: "local-2", Title: "Win", Runner: library.RunnerLocal,
		Platform: "Windows", Installed: true, Executable: "/games/win/game.exe",
		Metadata: map[string]string{"args": "-windowed"},
	}
	plan, err := Build(g, FactorySettings(config.DefaultsSettings{}), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WineType != WinePlain {
		t.Fatalf("wine type: %s", plan.WineType)
	}
	if strings.Join(plan.Argv, " ") == "" || !strings.Contains(strings.Join(plan.Argv, " "), "/games/win/game.exe -windowed") {
		t.Fatalf("argv: %q", plan.Argv)
	}
	if plan.Env["WINEPREFIX"] == "" {
		t.Fatal("wineprefix missing")
	}
}

func TestBuildNativeLinuxGame(t *testing.T) {
	withFakeBins(t, "gogdl")
	eng, paths := testManager(t)
	g := library.Game{
		Store: library.StoreGOG, ID: "1", Title: "Native", Platform: "Linux",
		Installed: true, InstallPath: "/games/native", Runner: library.RunnerGogdl,
	}
	plan, err := Build(g, FactorySettings(config.DefaultsSettings{}), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WineType != WineNative {
		t.Fatalf("wine type = %s, want native", plan.WineType)
	}
	if plan.Env["WINEPREFIX"] != "" {
		t.Error("native games must not get a wineprefix")
	}
}

func TestBuildProtonWithoutUmuFallsBackToDirectProton(t *testing.T) {
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	withFakeBins(t, "legendary")
	eng := engine.NewManager(config.EnginesSettings{UmuBin: "/nonexistent/umu-run"}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	plan, err := Build(epicGame(), protonSettings(), paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WineType != WineProtonDirect {
		t.Fatalf("wine type = %s, want proton", plan.WineType)
	}
	want := "launch b1a2c3d4e5f6g7h8i9j0 --no-wine --wrapper /usr/share/steam/compatibilitytools.d/proton-fake/proton waitforexitandrun"
	if got := strings.Join(plan.Argv[1:], " "); got != want {
		t.Fatalf("argv tail = %q, want %q", got, want)
	}
}

func TestBuildNoWineConfiguredIsActionable(t *testing.T) {
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	if err := paths.Ensure(); err != nil {
		t.Fatal(err)
	}
	withFakeBins(t, "legendary")
	eng := engine.NewManager(config.EnginesSettings{WineBin: "/nonexistent/wine", UmuBin: "/nonexistent/umu-run"}, engine.NewRunner(slog.New(slog.DiscardHandler)))
	_, err := Build(epicGame(), FactorySettings(config.DefaultsSettings{}), paths, eng, Options{})
	if err == nil || !strings.Contains(err.Error(), "no wine configured") {
		t.Fatalf("want actionable error, got %v", err)
	}
}

func TestEffectiveSettingsLayering(t *testing.T) {
	g := epicGame()
	imported := config.GameSettings{WinePrefix: "/pfx/imported", UseGameMode: true}

	s := EffectiveSettings(g, config.DefaultsSettings{}, nil, &imported)
	if s.WinePrefix != "/pfx/imported" || !s.UseGameMode {
		t.Fatalf("import layer lost: %+v", s)
	}
	if !s.EnableEsync {
		t.Fatal("factory esync default lost")
	}

	entry := &config.GameEntry{Settings: config.GameSettings{WinePrefix: "/pfx/mine"}}
	s = EffectiveSettings(g, config.DefaultsSettings{}, entry, &imported)
	if s.WinePrefix != "/pfx/mine" {
		t.Fatalf("user layer should win: %+v", s)
	}
	if !s.UseGameMode {
		t.Fatal("import flag lost under user layer")
	}
}

func TestBuildUmuIDOverridesGameID(t *testing.T) {
	withFakeBins(t, "umu-run", "legendary")
	eng, paths := testManager(t)
	plan, err := Build(epicGame(), protonSettings(), paths, eng, Options{UmuID: "umu-777"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Env["GAMEID"] != "umu-777" {
		t.Fatalf("GAMEID = %q, want umu-777", plan.Env["GAMEID"])
	}
}

func TestManagerLaunchStopLifecycle(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-engine")
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := engine.NewRunner(slog.New(slog.DiscardHandler))
	m := NewManager(slog.New(slog.DiscardHandler), runner)

	states := make(chan string, 8)
	plan := &Plan{Argv: []string{bin}, Env: map[string]string{"HIKARI_TEST": "1"}}
	if err := m.Launch(context.Background(), "epic", "x", plan, func(state string, pid int) {
		if state == StatePlaying && pid == 0 {
			t.Error("playing state must carry the pid")
		}
		states <- state
	}); err != nil {
		t.Fatal(err)
	}
	if got := <-states; got != StateLaunching {
		t.Fatalf("want launching, got %s", got)
	}
	if got := <-states; got != StatePlaying {
		t.Fatalf("want playing after start, got %s", got)
	}
	if !m.IsRunning("epic/x") {
		t.Fatal("game should be running")
	}
	running := m.Running()
	if len(running) != 1 || running[0].Store != "epic" || running[0].ID != "x" ||
		running[0].Pid == 0 || running[0].State != StatePlaying {
		t.Fatalf("running snapshot wrong: %+v", running)
	}
	if time.Since(running[0].StartedAt) > 5*time.Second {
		t.Fatalf("started_at not recent: %v", running[0].StartedAt)
	}
	if err := m.Launch(context.Background(), "epic", "x", plan, nil); err == nil {
		t.Fatal("double launch should fail")
	}
	if err := m.Stop("epic/x"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-states:
		if got != StateStopped {
			t.Fatalf("want stopped, got %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no exit callback")
	}
	if m.IsRunning("epic/x") {
		t.Fatal("game should be gone after stop")
	}
	if err := m.Stop("epic/x"); err == nil {
		t.Fatal("stopping a stopped game should fail")
	}
}

func TestSidecarDiesWithTheSession(t *testing.T) {
	dir := t.TempDir()
	mainBin := writeScriptFile(t, dir, "game", "#!/bin/sh\nsleep 30\n")
	sidecarPidFile := filepath.Join(dir, "sidecar.pid")
	sidecar := writeScriptFile(t, dir, "comet", "#!/bin/sh\necho $$ > "+sidecarPidFile+"\nsleep 30\n")

	runner := engine.NewRunner(slog.New(slog.DiscardHandler))
	m := NewManager(slog.New(slog.DiscardHandler), runner)
	plan := &Plan{
		Argv:     []string{mainBin},
		Sidecars: []Sidecar{{Name: "comet", Argv: []string{sidecar}}},
	}
	states := make(chan string, 4)
	if err := m.Launch(context.Background(), "gog", "x", plan, func(state string, pid int) {
		states <- state
	}); err != nil {
		t.Fatal(err)
	}
	<-states
	<-states
	var pid string
	for i := 0; i < 100; i++ {
		if b, err := os.ReadFile(sidecarPidFile); err == nil && len(b) > 0 {
			pid = strings.TrimSpace(string(b))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == "" {
		t.Fatal("sidecar never started")
	}
	if err := m.Stop("gog/x"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat("/proc/" + pid); os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sidecar %s survived the session", pid)
}

func writeScriptFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPlanEnvDiff(t *testing.T) {
	p := &Plan{Env: map[string]string{"B": "2", "A": "1"}}
	if diff := p.EnvDiff(); diff != "A=1\nB=2" {
		t.Fatalf("env diff = %q", diff)
	}
}
