package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppPayloadDirSitsBesideTheDataDir(t *testing.T) {
	t.Setenv("HIKARI_APP_PAYLOAD", "")
	d := &daemon{}
	d.paths.DataDir = "/home/someone/.local/share/hikari"
	if got := d.appPayloadDir(); got != "/home/someone/.local/share/hikari/app" {
		t.Fatalf("payload dir = %q", got)
	}
}

func TestAppPayloadDirHonoursTheOverride(t *testing.T) {
	t.Setenv("HIKARI_APP_PAYLOAD", "/tmp/payload")
	d := &daemon{}
	if got := d.appPayloadDir(); got != "/tmp/payload" {
		t.Fatalf("payload dir = %q", got)
	}
}

func TestAppDirPathPrefersTheEnvironment(t *testing.T) {
	t.Setenv("HIKARI_APPIMAGE_DIR", "/tmp/.mount_hikari/")
	if got := appDirPath(); got != "/tmp/.mount_hikari/" {
		t.Fatalf("appDirPath = %q", got)
	}
	t.Setenv("HIKARI_APPIMAGE_DIR", "")
	t.Setenv("APPDIR", "/tmp/.mount_other/")
	if got := appDirPath(); got != "/tmp/.mount_other/" {
		t.Fatalf("appDirPath = %q", got)
	}
	t.Setenv("APPDIR", "")
	if got := appDirPath(); got != "" && !fileExists(filepath.Join(got, "usr", "bin")) {
		t.Fatalf("appDirPath invented %q", got)
	}
	_ = os.Getenv("HOME")
}
