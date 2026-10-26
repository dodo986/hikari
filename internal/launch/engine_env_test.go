package launch

import (
	"path/filepath"
	"testing"

	"hikari/internal/config"
	"hikari/internal/library"
)

func TestPlanCarriesEngineConfigPath(t *testing.T) {
	binDir := withFakeBins(t, "wine", "legendary", "gogdl", "nile")
	eng, paths := testManager(t)

	cases := []struct {
		runner string
		key    string
		want   string
	}{
		{library.RunnerLegendary, "LEGENDARY_CONFIG_PATH", paths.EngineEpic},
		{library.RunnerGogdl, "GOGDL_CONFIG_PATH", paths.EngineGOG},
		{library.RunnerNile, "NILE_CONFIG_PATH", paths.EngineAmazon},
	}
	for _, c := range cases {
		g := epicGame()
		g.Runner = c.runner
		s := FactorySettings(config.DefaultsSettings{})
		s.WineVersion.Bin = filepath.Join(binDir, "wine")
		plan, err := Build(g, s, paths, eng, Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.runner, err)
		}
		if got := plan.Env[c.key]; got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.runner, c.key, got, c.want)
		}
	}
}

func TestPlanPointsAtRuntimeDirs(t *testing.T) {
	withFakeBins(t, "legendary")
	eng, paths := testManager(t)
	s := protonSettings()
	s.EacRuntime = true
	s.BattlEyeRuntime = true
	plan, err := Build(epicGame(), s, paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := plan.Env["PROTON_EAC_RUNTIME"], paths.DataDir+"/runtimes/eac_runtime"; got != want {
		t.Errorf("PROTON_EAC_RUNTIME = %q, want %q", got, want)
	}
	if got, want := plan.Env["PROTON_BATTLEYE_RUNTIME"], paths.DataDir+"/runtimes/battleye_runtime"; got != want {
		t.Errorf("PROTON_BATTLEYE_RUNTIME = %q, want %q", got, want)
	}
}

func TestPlanEnablesNvapiForPlainWine(t *testing.T) {
	withFakeBins(t, "legendary", "wine")
	eng, paths := testManager(t)
	s := FactorySettings(config.DefaultsSettings{})
	s.WineVersion = config.WineVersion{Bin: "/usr/bin/wine", Type: "wine"}
	s.AutoInstallDxvkNvapi = true
	plan, err := Build(epicGame(), s, paths, eng, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Env["DXVK_ENABLE_NVAPI"] != "1" || plan.Env["DXVK_NVAPI_ALLOW_OTHER_DRIVERS"] != "1" {
		t.Fatalf("nvapi env missing: %v", plan.Env)
	}
}
