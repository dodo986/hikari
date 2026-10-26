package launch

import (
	"path/filepath"
	"testing"

	"hikari/internal/config"
	"hikari/internal/library"
)

func TestPrefixSlug(t *testing.T) {
	cases := []struct {
		title, id, want string
	}{
		{"Sushi Racer", "id1", "Sushi-Racer"},
		{"Rocket League\u00ae", "id2", "Rocket-League"},
		{"Tom Clancy\u0027s  Rainbow Six", "id3", "Tom-Clancys-Rainbow-Six"},
		{"", "abc123", "abc123"},
		{"\u00ae\u00ae\u00ae", "abc123", "abc123"},
		{"Sid Meier\u0027s Civilization VI", "id4", "Sid-Meiers-Civilization-VI"},
	}
	for _, c := range cases {
		got := PrefixSlug(library.Game{Title: c.title, ID: c.id})
		if got != c.want {
			t.Errorf("PrefixSlug(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestEffectiveSettingsPerGamePrefix(t *testing.T) {
	g := epicGame()
	want := filepath.Join(DefaultPrefixDir(), "Sushi-Racer")
	if s := EffectiveSettings(g, config.DefaultsSettings{}, nil, nil); s.WinePrefix != want {
		t.Fatalf("default prefix = %q, want %q", s.WinePrefix, want)
	}

	if s := EffectiveSettings(g, config.DefaultsSettings{WinePrefix: "/pfx/shared"}, nil, nil); s.WinePrefix != "/pfx/shared" {
		t.Fatalf("global default lost: %q", s.WinePrefix)
	}

	imported := config.GameSettings{WinePrefix: "/pfx/imported"}
	if s := EffectiveSettings(g, config.DefaultsSettings{}, nil, &imported); s.WinePrefix != "/pfx/imported" {
		t.Fatalf("imported prefix lost: %q", s.WinePrefix)
	}
	entry := &config.GameEntry{Settings: config.GameSettings{WinePrefix: "/pfx/mine"}}
	if s := EffectiveSettings(g, config.DefaultsSettings{}, entry, &imported); s.WinePrefix != "/pfx/mine" {
		t.Fatalf("user prefix lost: %q", s.WinePrefix)
	}
}

func TestDropDefaultPrefix(t *testing.T) {
	g := epicGame()
	def := GamePrefixDir(g)

	cases := []struct {
		name     string
		defaults config.DefaultsSettings
		prefix   string
		want     string
	}{
		{"computed default dropped", config.DefaultsSettings{}, def, ""},
		{"trailing slash is the same path", config.DefaultsSettings{}, def + "/", ""},
		{"chosen path kept", config.DefaultsSettings{}, "/mnt/games/pfx", "/mnt/games/pfx"},
		{"copy of a global default kept", config.DefaultsSettings{WinePrefix: "/pfx/shared"}, def, def},
	}
	for _, c := range cases {
		s := config.GameSettings{WinePrefix: c.prefix}
		DropDefaultPrefix(g, c.defaults, &s)
		if s.WinePrefix != c.want {
			t.Errorf("%s: prefix = %q, want %q", c.name, s.WinePrefix, c.want)
		}
	}

	DropDefaultPrefix(g, config.DefaultsSettings{}, nil)
}

func TestUserEntryClearsInheritedBooleans(t *testing.T) {
	g := epicGame()
	imported := config.GameSettings{EacRuntime: true, EnableEsync: true}
	entry := &config.GameEntry{SettingsComplete: true, Settings: config.GameSettings{
		WinePrefix:  "/pfx/mine",
		EacRuntime:  false,
		EnableEsync: false,
	}}

	s := EffectiveSettings(g, config.DefaultsSettings{}, entry, &imported)
	if s.EacRuntime {
		t.Error("entry false did not clear the imported EAC runtime")
	}
	if s.EnableEsync {
		t.Error("entry false did not clear the factory esync default")
	}

	if s := EffectiveSettings(g, config.DefaultsSettings{}, nil, &imported); !s.EacRuntime || !s.EnableEsync {
		t.Fatalf("imported booleans lost without an entry: eac=%v esync=%v", s.EacRuntime, s.EnableEsync)
	}
}
