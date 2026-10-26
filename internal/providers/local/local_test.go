package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hikari/internal/library"
)

func TestAddListRemove(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Game", "game.exe")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("MZ"), 0o755)
	reg := filepath.Join(t.TempDir(), "local_games.json")

	p := New(reg)
	g, err := p.Add(Spec{Title: "Sushi Racer", Executable: exe, Platform: "Windows", Args: []string{"-windowed"}})
	if err != nil {
		t.Fatal(err)
	}
	if g.Store != library.StoreLocal || g.Runner != library.RunnerLocal || !g.Installed {
		t.Fatalf("bad game: %+v", g)
	}
	if g.Metadata["args"] != "-windowed" {
		t.Fatalf("args not kept: %+v", g.Metadata)
	}
	games, err := p.List(context.Background())
	if err != nil || len(games) != 1 || games[0].ID != g.ID {
		t.Fatalf("list: %+v %v", games, err)
	}
	if _, err := p.Add(Spec{Title: "Again", Executable: exe, Platform: "Windows"}); err == nil {
		t.Fatal("duplicate should fail")
	}
	cases := []Spec{
		{Title: "", Executable: exe, Platform: "Windows"},
		{Title: "x", Executable: "relative/path", Platform: "Windows"},
		{Title: "x", Executable: filepath.Join(dir, "missing"), Platform: "Windows"},
		{Title: "x", Executable: exe, Platform: "Mac"},
	}
	for _, c := range cases {
		if _, err := p.Add(c); err == nil {
			t.Fatalf("should reject %+v", c)
		}
	}
	p2 := New(reg)
	if games, _ := p2.List(context.Background()); len(games) != 1 {
		t.Fatal("registry not persisted")
	}
	if err := p2.Remove(g.ID); err != nil {
		t.Fatal(err)
	}
	if games, _ := p2.List(context.Background()); len(games) != 0 {
		t.Fatalf("remove failed: %+v", games)
	}
	if err := p2.Remove(g.ID); err == nil {
		t.Fatal("removing twice should fail")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatal("remove must not delete files")
	}
}

func TestMissingExecutableShowsUninstalled(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "gone")
	os.WriteFile(exe, []byte("x"), 0o755)
	p := New(filepath.Join(t.TempDir(), "reg.json"))
	g, err := p.Add(Spec{Title: "Gone", Executable: exe, Platform: "Linux"})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(exe)
	games, _ := p.List(context.Background())
	if len(games) != 1 || games[0].ID != g.ID || games[0].Installed {
		t.Fatalf("missing exe should list as not installed: %+v", games)
	}
}

func TestDetectPlatform(t *testing.T) {
	dir := t.TempDir()
	write := func(name, magic string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(magic), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	tests := []struct {
		name, magic, want string
	}{
		{"game.exe", "garbage", "Windows"},
		{"game", "MZ\x90\x00", "Windows"},
		{"game.bin", "\x7fELF", "Linux"},
		{"run.sh", "#!/bin/sh\n", "Linux"},
		{"data.dat", "hello", ""},
	}
	for _, tc := range tests {
		got := DetectPlatform(write(tc.name, tc.magic))
		if got != tc.want {
			t.Errorf("%s: DetectPlatform = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := DetectPlatform(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("missing file: got %q, want empty", got)
	}
}

func TestPlatformFollowsTheExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Cyberpunk", "REDprelauncher.exe")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("MZ\x90\x00"), 0o755)
	reg := filepath.Join(t.TempDir(), "local_games.json")

	g, err := New(reg).Add(Spec{Title: "Cyberpunk", Executable: exe, Platform: "Linux"})
	if err != nil {
		t.Fatal(err)
	}
	if g.Platform != "Windows" {
		t.Fatalf("Add kept platform %q, want Windows", g.Platform)
	}

	stale := `[{"id":"local-9d90d01f","spec":{"title":"Cyberpunk",` +
		`"executable":"` + exe + `","platform":"Linux"}}]`
	if err := os.WriteFile(reg, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	games, err := New(reg).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].Platform != "Windows" {
		t.Fatalf("List = %+v, want one Windows game", games)
	}
}
