package launch

import (
	"strings"
	"testing"

	"hikari/internal/library"
)

// Mods that ship a proxy DLL (CET's version.dll, MelonLoader's winmm.dll) only
// load when Wine is told to prefer the native copy; without this the game
// starts and the mod silently never appears.
func TestModOverridesReachTheLaunchEnv(t *testing.T) {
	withFakeBins(t, "umu-run", "legendary", "gogdl", "nile")
	eng, paths := testManager(t)
	opts := Options{ModOverrides: map[string]string{
		"version": "native,builtin",
		"winmm":   "native,builtin",
	}}
	plan, err := Build(epicGame(), protonSettings(), paths, eng, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := "version=native,builtin;winmm=native,builtin"
	if plan.Env["WINEDLLOVERRIDES"] != want {
		t.Fatalf("WINEDLLOVERRIDES = %q, want %q", plan.Env["WINEDLLOVERRIDES"], want)
	}
}

func TestModOverridesKeepWhatTheGameAlreadyAskedFor(t *testing.T) {
	env := map[string]string{"WINEDLLOVERRIDES": "mshtml=d;version=builtin"}
	MergeDLLOverrides(env, map[string]string{"version": "native,builtin"})
	if env["WINEDLLOVERRIDES"] != "mshtml=d;version=native,builtin" {
		t.Fatalf("WINEDLLOVERRIDES = %q", env["WINEDLLOVERRIDES"])
	}
	empty := map[string]string{}
	MergeDLLOverrides(empty, nil)
	if _, ok := empty["WINEDLLOVERRIDES"]; ok {
		t.Fatal("no overrides should not invent WINEDLLOVERRIDES")
	}
}

// A local game started with a relative target exe belongs in its own folder;
// running it from the daemon's directory left wine caches and crash dumps
// wherever the daemon happened to be started.
func TestLocalGameRunsInItsOwnDirectory(t *testing.T) {
	withFakeBins(t, "umu-run", "legendary", "gogdl", "nile")
	eng, paths := testManager(t)
	game := library.Game{
		Store: library.StoreLocal, ID: "local-abc", Title: "Rip",
		Platform: "Windows", Installed: true, Runner: library.RunnerLocal,
		InstallPath: "/games/rip",
	}
	settings := protonSettings()
	settings.TargetExe = "bin/x64/rip.exe"
	plan, err := Build(game, settings, paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cwd != "/games/rip/bin/x64" {
		t.Fatalf("cwd = %q", plan.Cwd)
	}
	if !strings.HasSuffix(plan.Argv[len(plan.Argv)-1], "/games/rip/bin/x64/rip.exe") {
		t.Fatalf("argv = %q", plan.Argv)
	}
}
