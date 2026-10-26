package launch

import (
	"os"
	"path/filepath"
	"testing"

	"hikari/internal/config"
)

func fakeWineTree(t *testing.T, withLib32, withLib64 bool) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin", "wine")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tree := range []struct {
		name string
		on   bool
	}{{"lib", withLib32}, {"lib64", withLib64}} {
		if !tree.on {
			continue
		}
		for _, sub := range []string{"gstreamer-1.0", "wine"} {
			if err := os.MkdirAll(filepath.Join(root, tree.name, sub), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	return bin
}

func TestSetupWineLibEnv(t *testing.T) {
	t.Setenv("LD_LIBRARY_PATH", "/sys/lib")

	bin := fakeWineTree(t, true, true)
	root := filepath.Dir(filepath.Dir(bin))
	env := map[string]string{"WINEPREFIX": "/pfx"}
	SetupWineLibEnv(config.GameSettings{}, WinePlain, bin, env)

	wantLibs := filepath.Join(root, "lib64") + ":" + filepath.Join(root, "lib") + ":/sys/lib"
	if got := env["LD_LIBRARY_PATH"]; got != wantLibs {
		t.Fatalf("LD_LIBRARY_PATH = %q, want %q", got, wantLibs)
	}
	if got := env["ORIG_LD_LIBRARY_PATH"]; got != "/sys/lib" {
		t.Fatalf("ORIG_LD_LIBRARY_PATH = %q, want the inherited value", got)
	}
	if got, want := env["GST_PLUGIN_SYSTEM_PATH_1_0"], filepath.Join(root, "lib64", "gstreamer-1.0")+":"+filepath.Join(root, "lib", "gstreamer-1.0"); got != want {
		t.Fatalf("GST_PLUGIN_SYSTEM_PATH_1_0 = %q, want %q", got, want)
	}
	if got, want := env["WINEDLLPATH"], filepath.Join(root, "lib64", "wine")+":"+filepath.Join(root, "lib", "wine"); got != want {
		t.Fatalf("WINEDLLPATH = %q, want %q", got, want)
	}
}

func TestSetupWineLibEnvSkips(t *testing.T) {
	t.Setenv("LD_LIBRARY_PATH", "/sys/lib")
	bin := fakeWineTree(t, true, true)

	cases := []struct {
		name     string
		settings config.GameSettings
		wineType string
		bin      string
	}{
		{"system libs requested", config.GameSettings{PreferSystemLibs: true}, WinePlain, bin},
		{"proton", config.GameSettings{}, WineProtonUmu, bin},
		{"no wine binary", config.GameSettings{}, WinePlain, ""},
		{"no lib32 tree", config.GameSettings{}, WinePlain, fakeWineTree(t, false, true)},
	}
	for _, c := range cases {
		env := map[string]string{}
		SetupWineLibEnv(c.settings, c.wineType, c.bin, env)
		if len(env) != 0 {
			t.Errorf("%s: env should stay empty, got %v", c.name, env)
		}
	}
}

func TestBuildPlainWineSetsLibraryPath(t *testing.T) {
	bin := fakeWineTree(t, true, true)
	withFakeBins(t, "legendary")
	eng, paths := testManager(t)

	s := FactorySettings(config.DefaultsSettings{})
	s.WineVersion = config.WineVersion{Bin: bin, Type: "wine"}
	plan, err := Build(epicGame(), s, paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Env["LD_LIBRARY_PATH"] == "" || plan.Env["WINEDLLPATH"] == "" {
		t.Fatalf("plain wine plan lost the library env: %v", plan.Env)
	}
	if plan.Argv[len(plan.Argv)-2] != "--wine" {
		t.Fatalf("plain wine must still pass --wine, argv = %v", plan.Argv)
	}
}
