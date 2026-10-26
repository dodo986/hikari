package opti

import (
	"archive/tar"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGpuNameAndArchClassification(t *testing.T) {
	cases := []struct {
		line   string
		name   string
		vendor string
		arch   string
	}{
		{"0000:03:00.0 VGA compatible controller [0300]: NVIDIA Corporation AD103 [GeForce RTX 4090] (rev a1)",
			"GeForce RTX 4090", "NVIDIA", "rtx"},
		{"01:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Navi 31 [Radeon RX 7900 XT/XTX]",
			"Radeon RX 7900 XT/XTX", "AMD", "rdna1-3"},
		{"01:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Navi 48 [Radeon RX 9070 XT]",
			"Radeon RX 9070 XT", "AMD", "rdna4"},
		{"00:02.0 VGA compatible controller: Intel Corporation Alder Lake-S GT1 [UHD Graphics 770]",
			"UHD Graphics 770", "Intel", "other-intel"},
		{"0000:01:00.0 3D controller [0302]: Intel Corporation DG2 [Arc A770M] (rev 08)",
			"Arc A770M", "Intel", "intel-arc"},
		{"05:00.0 VGA compatible controller: NVIDIA Corporation TU116 [GeForce GTX 1660] (rev a1)",
			"GeForce GTX 1660", "NVIDIA", "gtx"},
	}
	for _, c := range cases {
		name := gpuNameFromLine(c.line)
		if name != c.name {
			t.Errorf("name: got %q want %q", name, c.name)
			continue
		}
		if v := vendorOf(name); v != c.vendor {
			t.Errorf("%s: vendor %q want %q", name, v, c.vendor)
		}
		if a := archOf(name); a != c.arch {
			t.Errorf("%s: arch %q want %q", name, a, c.arch)
		}
	}
}

func TestIniSetAndRead(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "OptiScaler.ini")
	content := "; comment\n[Upscalers]\n; pick one\nDx11Upscaler=auto\nDx12Upscaler=auto\n\n[FrameGen]\nEnabled=auto\n"
	if err := os.WriteFile(ini, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := IniSet(ini, "Upscalers", "Dx11Upscaler", "fsr22_12"); err != nil {
		t.Fatal(err)
	}
	if got := IniGet(ini, "Upscalers", "Dx11Upscaler"); got != "fsr22_12" {
		t.Fatalf("read back %q", got)
	}
	if got := IniGet(ini, "FrameGen", "Enabled"); got != "auto" {
		t.Fatalf("framegen touched: %q", got)
	}
	if err := IniSet(ini, "FrameGen", "FGInput", "fsrfg"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(ini)
	s := string(b)
	if !strings.Contains(s, "FGInput=fsrfg\n") || strings.Index(s, "FGInput=fsrfg") < strings.Index(s, "[FrameGen]") {
		t.Fatalf("insert misplaced:\n%s", s)
	}
	if !strings.Contains(s, "; comment") {
		t.Fatal("comments must survive")
	}
	if err := IniSet(ini, "NoSuchSection", "k", "v"); err == nil {
		t.Fatal("unknown section must error")
	}
}

func fakeReleaseDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"OptiScaler.dll":                                    "payload",
		"OptiScaler.ini":                                    "[Upscalers]\nDx11Upscaler=auto\nDx12Upscaler=auto\nVulkanUpscaler=auto\n[Spoofing]\nDxgi=auto\n[FrameGen]\nEnabled=auto\nFGInput=auto\nFGOutput=auto\n",
		"!! EXTRACT ALL FILES TO GAME FOLDER !!":            "",
		"!! README_EXTRACT ALL FILES TO GAME FOLDER !!.txt": "readme",
		"setup_windows.bat":                                 "@echo off",
		"setup_linux.sh":                                    "#!/bin/sh",
		"fakenvapi.dll":                                     "nvapi",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCopyConfigureUninstallCycle(t *testing.T) {
	versionDir := fakeReleaseDir(t)
	gameDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(gameDir, "dxgi.dll"), []byte("original dxgi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyVersionTree(versionDir, gameDir, "dxgi.dll"); err != nil {
		t.Fatal(err)
	}
	hook, err := os.ReadFile(filepath.Join(gameDir, "dxgi.dll"))
	if err != nil || string(hook) != "payload" {
		t.Fatalf("hook dll not installed: %v %q", err, hook)
	}
	backup, err := os.ReadFile(filepath.Join(gameDir, BackupDir, "dxgi.dll"))
	if err != nil || string(backup) != "original dxgi" {
		t.Fatalf("original hook not backed up: %v", err)
	}
	for _, junk := range []string{"setup_windows.bat", "setup_linux.sh",
		"!! EXTRACT ALL FILES TO GAME FOLDER !!",
		"!! README_EXTRACT ALL FILES TO GAME FOLDER !!.txt"} {
		if _, err := os.Stat(filepath.Join(gameDir, junk)); !os.IsNotExist(err) {
			t.Fatalf("junk file copied: %s", junk)
		}
	}
	if _, err := os.Stat(filepath.Join(gameDir, "fakenvapi.dll")); err != nil {
		t.Fatal("release file missing")
	}
	if _, ok := ReadManifest(gameDir); !ok {
		t.Fatal("manifest missing")
	}

	if err := ConfigureINI(gameDir, InstallOptions{SpoofDLSS: true}, GPU{Vendor: "AMD", Architecture: "rdna1-3"}); err != nil {
		t.Fatal(err)
	}
	ini := filepath.Join(gameDir, "OptiScaler.ini")
	if IniGet(ini, "Upscalers", "Dx12Upscaler") != "fsr31" {
		t.Fatalf("amd dx12 default: %q", IniGet(ini, "Upscalers", "Dx12Upscaler"))
	}
	if IniGet(ini, "Spoofing", "Dxgi") != "auto" {
		t.Fatalf("spoof should stay on: %q", IniGet(ini, "Spoofing", "Dxgi"))
	}
	if err := ConfigureINI(gameDir, InstallOptions{}, GPU{Vendor: "NVIDIA", Architecture: "rtx"}); err != nil {
		t.Fatal(err)
	}
	if IniGet(ini, "Upscalers", "Dx11Upscaler") != "dlss" {
		t.Fatalf("rtx dx11 default: %q", IniGet(ini, "Upscalers", "Dx11Upscaler"))
	}
	if err := ConfigureINI(gameDir, InstallOptions{FrameGen: true, FGInput: "fsrfg"}, GPU{Vendor: "AMD", Architecture: "rdna1-3"}); err != nil {
		t.Fatal(err)
	}
	if IniGet(ini, "FrameGen", "Enabled") != "true" || IniGet(ini, "FrameGen", "FGInput") != "fsrfg" {
		t.Fatal("framegen keys not written")
	}

	if err := RemoveFromGameDir(gameDir); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(gameDir, "dxgi.dll"))
	if err != nil || string(restored) != "original dxgi" {
		t.Fatalf("original hook not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "OptiScaler.ini")); !os.IsNotExist(err) {
		t.Fatal("ini should be removed")
	}
	if _, err := os.Stat(filepath.Join(gameDir, "fakenvapi.dll")); !os.IsNotExist(err) {
		t.Fatal("release extras should be removed")
	}
	if err := RemoveFromGameDir(gameDir); err == nil {
		t.Fatal("uninstall without manifest must refuse")
	}
}

func TestLocalManagerEndToEnd(t *testing.T) {
	versionDir := fakeReleaseDir(t)
	tgz := filepath.Join(t.TempDir(), "Optiscaler_0.9.4.7z")
	if err := tarGz(tgz, versionDir); err != nil {
		t.Fatal(err)
	}
	archiveBytes, _ := os.ReadFile(tgz)
	digest := "sha256:" + sha256Hex(archiveBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases"):
			w.Write([]byte(`[{"tag_name":"v0.9.4","html_url":"https://github.com/optiscaler/OptiScaler/releases/tag/v0.9.4","published_at":"2026-07-18T00:00:00Z"}]`))
		case strings.Contains(r.URL.Path, "/tags/"):
			w.Write([]byte(`{"assets":[{"name":"Optiscaler_0.9.4.7z","browser_download_url":"` + srvURL(r) + `/asset","size":` + itoa(len(archiveBytes)) + `,"digest":"` + digest + `"}]}`))
		case r.URL.Path == "/asset":
			w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	store := t.TempDir()
	m := NewLocalManager(store, NewGitHubClient())
	m.gh.base = srv.URL + "/releases"
	m.extract = ExtractTgz

	ctx := context.Background()
	releases, err := m.ListReleases(ctx)
	if err != nil || len(releases) != 1 || releases[0].Tag != "v0.9.4" || releases[0].Date != "2026-07-18" {
		t.Fatalf("releases: %+v %v", releases, err)
	}

	var frames []Progress
	if err := m.Download(ctx, "v0.9.4", func(p Progress) { frames = append(frames, p) }); err != nil {
		t.Fatal(err)
	}
	if len(frames) == 0 || !frames[len(frames)-1].Done {
		t.Fatalf("no done frame: %+v", frames)
	}
	if v, _ := m.ListVersions(ctx); len(v) != 1 || v[0] != "v0.9.4" {
		t.Fatalf("versions: %v", v)
	}

	gameDir := t.TempDir()
	err = m.Install(ctx, InstallOptions{
		Version: "v0.9.4", ExePath: filepath.Join(gameDir, "Game.exe"),
		InstallDir: gameDir, Injection: "dxgi", SpoofDLSS: false,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	st, err := m.Status(ctx, "g1", filepath.Join(gameDir, "Game.exe"), gameDir)
	if err != nil || len(st) != 1 || !st[0].Installed || st[0].Version != "v0.9.4" || st[0].InjectionDLL != "dxgi.dll" {
		t.Fatalf("status: %+v %v", st, err)
	}
	if st[0].GameDir != gameDir {
		t.Fatalf("status dir: %q", st[0].GameDir)
	}
	if err := m.Uninstall(ctx, "g1", filepath.Join(gameDir, "Game.exe"), gameDir); err != nil {
		t.Fatal(err)
	}
	if st, _ := m.Status(ctx, "g1", filepath.Join(gameDir, "Game.exe"), gameDir); len(st) != 0 {
		t.Fatalf("status after uninstall: %+v", st)
	}
}

func TestDownloadVerifiesChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tags/") {
			w.Write([]byte(`{"assets":[{"name":"x.7z","browser_download_url":"` + srvURL(r) + `/asset","size":5,"digest":"sha256:` + sha256Hex([]byte("wrong")) + `"}]}`))
		} else {
			w.Write([]byte("right"))
		}
	}))
	defer srv.Close()
	m := NewLocalManager(t.TempDir(), NewGitHubClient())
	m.gh.base = srv.URL + "/releases"
	m.extract = ExtractTgz
	err := m.Download(context.Background(), "vX", nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum failure, got %v", err)
	}
	entries, _ := os.ReadDir(m.storeDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".download-") || strings.HasSuffix(e.Name(), ".7z") {
			t.Fatalf("leftover file: %s", e.Name())
		}
	}
}

func TestInstallRejectsBadInputs(t *testing.T) {
	m := NewLocalManager(t.TempDir(), NewGitHubClient())
	if err := m.Install(context.Background(), InstallOptions{}, nil); err == nil {
		t.Fatal("missing version must fail")
	}
	if err := m.Install(context.Background(), InstallOptions{Version: "v"}, nil); err == nil {
		t.Fatal("missing game dir must fail")
	}
	if !ValidInjection("dxgi.dll") || ValidInjection("evil.dll") {
		t.Fatal("injection validation broken")
	}
}

func tarGz(dest, dir string) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	return tarDir(f, dir)
}

func srvURL(r *http.Request) string {
	return "http://" + r.Host
}

func itoa(n int) string {
	return fmtInt(n)
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func tarDir(w *os.File, dir string) error {
	gz := gzipNew(w)
	defer gz.Close()
	tw := tarNew(gz)
	defer tw.Close()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: rel + "/", Mode: 0o755, Typeflag: tar.TypeDir})
		}
		hdr := &tar.Header{Name: rel, Mode: 0o644, Size: info.Size()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = ioCopy(tw, src)
		return err
	})
}

func sha256Hex(b []byte) string {
	sum := sha256Sum(b)
	return hexEncode(sum[:])
}
