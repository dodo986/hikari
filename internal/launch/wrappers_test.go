package launch

import (
	"reflect"
	"testing"

	"hikari/internal/config"
)

// A wrapper flag that carries a path with a space has to survive: the old
// strings.Fields split it into two arguments and a stray quote.
func TestWrappersKeepQuotedArguments(t *testing.T) {
	s := config.GameSettings{
		Wrappers: []config.Wrapper{{Exe: "/usr/bin/mangohud", Args: `--config "/home/me/my dir/cfg.ini" --dlsym`}},
	}
	got := Wrappers(s, "")
	want := []string{"/usr/bin/mangohud", "--config", "/home/me/my dir/cfg.ini", "--dlsym"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrappers = %#v, want %#v", got, want)
	}
}

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"a b", []string{"a", "b"}},
		{`'a b' c`, []string{"a b", "c"}},
		{`"a b" c`, []string{"a b", "c"}},
		{`a\ b c`, []string{"a b", "c"}},
		{`a "b\"c" d`, []string{"a", `b"c`, "d"}},
		{`'' x`, []string{"", "x"}},
		{`--flag=value`, []string{"--flag=value"}},
		{`trailing\`, []string{`trailing\`}},
		{"line\nbreak", []string{"line", "break"}},
	}
	for _, c := range cases {
		if got := splitArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

// A configured but missing gamescope must not silently drop MangoHud and the
// frame limiter: only a gamescope that is actually in the chain replaces them.
func TestMissingGamescopeDoesNotDisableMangoHud(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no gamescope anywhere
	s := config.GameSettings{
		ShowMangoHud: true,
		UseGameMode:  true,
		Gamescope:    &config.GamescopeSettings{EnableLimiter: true, FpsLimiter: "60"},
	}
	got := Wrappers(s, "")
	want := []string{"mangohud", "--dlsym", "gamemoderun"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrappers = %#v, want %#v", got, want)
	}
}
