package launch

import (
	"reflect"
	"testing"

	"hikari/internal/library"
)

// Local game args are stored as a JSON list now; space-joined values from older
// registries must keep working, and a quoted legacy value has to survive.
func TestLocalArgsFormats(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{`["-name","My Game"]`, []string{"-name", "My Game"}},
		{`-windowed -name "My Game"`, []string{"-windowed", "-name", "My Game"}},
		{`[""]`, []string{""}},
	}
	for _, c := range cases {
		game := library.Game{Metadata: map[string]string{"args": c.raw}}
		got, ok := localArgs(game)
		if !ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("localArgs(%q) = %q, %v; want %q", c.raw, got, ok, c.want)
		}
	}
	if _, ok := localArgs(library.Game{}); ok {
		t.Error("no args must report false")
	}
}
