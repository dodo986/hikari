package epic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledSizeShapes(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`12884901888`, "12.00 GiB"},
		{`"12884901888"`, "12.00 GiB"},
		{`null`, "0"},
		{`"12 GiB"`, "0"},
	}
	for _, c := range cases {
		p, paths := newTestProvider(t, fakeLegendary)
		seedMetadata(t, paths, "b1a2c3d4e5f6")
		inst := `{"b1a2c3d4e5f6":{"install_path":"/games/SushiRacer","version":"1.0.4","platform":"Windows","install_size":` + c.raw + `}}`
		if err := os.WriteFile(filepath.Join(paths.EngineEpic, "installed.json"), []byte(inst), 0o600); err != nil {
			t.Fatal(err)
		}
		games, err := p.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(games) != 1 || !games[0].Installed {
			t.Fatalf("install_size %s: want the installed game, got %+v", c.raw, games)
		}
		if got := games[0].InstallSize; got != c.want {
			t.Errorf("install_size %s: want %q, got %q", c.raw, c.want, got)
		}
	}
}
