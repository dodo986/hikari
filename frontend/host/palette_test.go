package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeColors(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".cache", "ryoku")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "colors.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPaletteStampTracksTheFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if paletteStamp() != "missing" {
		t.Fatalf("expected a missing file, got %q", paletteStamp())
	}
	writeColors(t, home, `{"primary":"#ff0000"}`)
	first := paletteStamp()
	if first == "missing" {
		t.Fatal("the file should be stamped once it exists")
	}
	writeColors(t, home, `{"primary":"#00ff00"}`)
	if paletteStamp() == first {
		t.Fatal("a rewritten palette should change the stamp")
	}
}

func TestReadPaletteReturnsRoles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if roles := readPalette(); roles != nil {
		t.Fatal("no file should read as no palette")
	}
	writeColors(t, home, `{"primary":"#00ff00","surface":"#101010"}`)
	roles, ok := readPalette().(map[string]any)
	if !ok {
		t.Fatalf("expected roles, got %T", readPalette())
	}
	if roles["primary"] != "#00ff00" {
		t.Fatalf("primary is %v", roles["primary"])
	}
}
