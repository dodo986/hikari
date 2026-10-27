package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJoinAppImageKeepsTheRuntimeAndAppendsTheSquashfs(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.AppImage")
	runtime := []byte("runtime-bytes-here")
	if err := os.WriteFile(old, append(append([]byte{}, runtime...), []byte("stale-squashfs")...), 0o755); err != nil {
		t.Fatal(err)
	}
	squashed := filepath.Join(dir, "root.squashfs")
	if err := os.WriteFile(squashed, []byte("fresh-squashfs"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "new.AppImage")
	if err := joinAppImage(old, out, int64(len(runtime)), squashed); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "runtime-bytes-herefresh-squashfs" {
		t.Fatalf("new AppImage is %q", got)
	}
}

func TestAppImageOffsetRejectsWhatIsNotAnAppImage(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "not.AppImage")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := appImageOffset(t.Context(), plain); err == nil {
		t.Fatal("a file without an AppImage runtime must not report an offset")
	}
}

func TestAppDirPathFallsBackToTheRunningBinary(t *testing.T) {
	t.Setenv("HIKARI_APPIMAGE_DIR", "/tmp/.mount_hikari/")
	if got := appDirPath(); got != "/tmp/.mount_hikari/" {
		t.Fatalf("appDirPath = %q", got)
	}
	t.Setenv("HIKARI_APPIMAGE_DIR", "")
	t.Setenv("APPDIR", "")
	if got := appDirPath(); got == "" {
		t.Skip("the test binary is not inside an AppDir")
	}
}
