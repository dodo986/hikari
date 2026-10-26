package epic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hikari/internal/config"
	"hikari/internal/library"
)

func TestRealMetadataIfPresent(t *testing.T) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "hikari", "engines", "legendary", "metadata")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Skip("no legendary metadata in the engine state")
	}
	paths := config.ResolveFrom(filepath.Join(home, ".config"), filepath.Join(home, ".local", "share"))
	p := New(nil, nil, paths)
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var mf metadataFile
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if err := jsonUnmarshal(b, &mf); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		app := strings.TrimSuffix(e.Name(), ".json")
		g, ok := p.toGame(app, mf, nil)
		if !ok {
			continue
		}
		n++
		if g.Title == "" {
			t.Errorf("%s: title empty", app)
		}
		if g.Artwork.Square == "" && g.Artwork.Cover == "" {
			t.Errorf("%s (%s): artwork empty though metadata has %d keyImages", app, g.Title, len(mf.Metadata.KeyImages))
		}
		if n > 0 && !g.Installed {
			_ = library.Game{}
		}
	}
	if n == 0 {
		t.Fatal("no games parsed from real metadata")
	}
	t.Logf("parsed %d real games with artwork", n)
}

func jsonUnmarshal(b []byte, v any) error { return jsonDecode(b, v) }
