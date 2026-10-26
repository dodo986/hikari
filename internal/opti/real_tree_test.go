package opti

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRealReleaseTreeIfPresent(t *testing.T) {
	dir := realReleaseDir()
	if dir == "" {
		t.Skip("no extracted OptiScaler release in the version store")
	}
	gameDir := t.TempDir()

	own := filepath.Join(gameDir, "dxgi.dll")
	if err := os.WriteFile(own, []byte("the game's real dxgi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyVersionTree(dir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	for _, junk := range []string{
		"!! README_EXTRACT ALL FILES TO GAME FOLDER !!.txt",
		"setup_windows.bat", "setup_linux.sh",
	} {
		if _, err := os.Stat(filepath.Join(gameDir, junk)); !os.IsNotExist(err) {
			t.Errorf("junk copied into game dir: %s", junk)
		}
	}
	if err := ConfigureINI(gameDir, InstallOptions{SpoofDLSS: true},
		GPU{Name: "Radeon RX 6700 XT", Vendor: "AMD", Architecture: "rdna1-3"}); err != nil {
		t.Fatal(err)
	}
	if got := IniGet(filepath.Join(gameDir, "OptiScaler.ini"), "Upscalers", "Dx12Upscaler"); got != "fsr31" {
		t.Errorf("amd dx12 upscaler = %q, want fsr31", got)
	}
	if err := RemoveFromGameDir(gameDir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(own)
	if err != nil || string(b) != "the game's real dxgi" {
		t.Fatalf("the game's own dxgi.dll did not come back: %v %q", err, b)
	}
}

func realReleaseDir() string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		data = filepath.Join(home, ".local", "share")
	}
	store := filepath.Join(data, "hikari", "optiscaler", "versions")
	entries, err := os.ReadDir(store)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(store, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "OptiScaler.dll")); err == nil {
			return dir
		}
	}
	return ""
}
