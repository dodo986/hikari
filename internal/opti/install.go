package opti

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var InjectionDLLs = []string{
	"dxgi.dll", "winmm.dll", "version.dll", "dbghelp.dll",
	"d3d12.dll", "wininet.dll", "winhttp.dll", "OptiScaler.asi",
}

var releaseJunk = []string{
	"!! EXTRACT ALL FILES TO GAME FOLDER !!",
	"!! README_EXTRACT ALL FILES TO GAME FOLDER !!.txt",
	"setup_windows.bat",
	"setup_linux.sh",
}

const BackupDir = "OptiScalerBackup"

const ManifestFile = "hikari-optiscaler.json"

type Manifest struct {
	Version      string `json:"version"`
	InjectionDLL string `json:"injection_dll"`
}

func HookName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "dxgi.dll"
	}
	if strings.Contains(name, ".") {
		return name
	}
	return name + ".dll"
}

func ValidInjection(name string) bool {
	for _, d := range InjectionDLLs {
		if strings.EqualFold(d, name) {
			return true
		}
	}
	return false
}

func CopyVersionTree(versionDir, gameDir, injectionDLL string) error {
	if !ValidInjection(injectionDLL) {
		return fmt.Errorf("%q is not an OptiScaler hook name", injectionDLL)
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return err
	}

	hook := filepath.Join(gameDir, injectionDLL)
	if _, err := os.Lstat(hook); err == nil {
		backup := filepath.Join(gameDir, BackupDir)
		if err := os.MkdirAll(backup, 0o755); err != nil {
			return err
		}
		bakTarget := filepath.Join(backup, injectionDLL)
		if _, err := os.Lstat(bakTarget); err != nil {
			if err := os.Rename(hook, bakTarget); err != nil {
				return fmt.Errorf("cannot back up existing %s: %w", injectionDLL, err)
			}
		}
	}

	err := filepath.Walk(versionDir, func(src string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(versionDir, src)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := strings.ToLower(info.Name())
		for _, junk := range releaseJunk {
			if strings.EqualFold(info.Name(), junk) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		dst := filepath.Join(gameDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if name == "optiscaler.dll" {
			return copyFile(src, hook)
		}
		return copyFile(src, dst)
	})
	if err != nil {
		return err
	}
	m, _ := json.MarshalIndent(Manifest{Version: filepath.Base(versionDir), InjectionDLL: injectionDLL}, "", "  ")
	return atomicWrite(filepath.Join(gameDir, ManifestFile), m)
}

func ReadManifest(gameDir string) (Manifest, bool) {
	b, err := os.ReadFile(filepath.Join(gameDir, ManifestFile))
	if err != nil {
		return Manifest{}, false
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil {
		return Manifest{}, false
	}
	return m, true
}

func RemoveFromGameDir(gameDir string) error {
	if _, ok := ReadManifest(gameDir); !ok {
		return fmt.Errorf("no Hikari OptiScaler install found in %s", gameDir)
	}
	m, _ := ReadManifest(gameDir)

	hook := filepath.Join(gameDir, m.InjectionDLL)
	backup := filepath.Join(gameDir, BackupDir, m.InjectionDLL)
	if _, err := os.Lstat(backup); err == nil {
		if err := os.Rename(backup, hook); err != nil {
			return err
		}
	} else if _, err := os.Lstat(hook); err == nil {
		if err := os.Remove(hook); err != nil {
			return err
		}
	}

	extras := []string{
		"OptiScaler.ini", "OptiScaler.log", "OptiScaler.log.old",
		"fakenvapi.dll", "fakenvapi.ini", "fakenvapi.log",
		"dlssg_to_fsr3_amd_is_better.dll", "dlss-enabler-headless.dll",
		"dlss-enabler-upscaler.dll", "GenFSR_DLSS",
		ManifestFile,
	}
	for _, name := range extras {
		_ = os.Remove(filepath.Join(gameDir, name))
	}
	for _, dir := range []string{"D3D12_Optiscaler", "DlssOverrides", "Licenses", BackupDir} {
		_ = os.RemoveAll(filepath.Join(gameDir, dir))
	}
	_ = os.Remove(filepath.Join(gameDir, "GenFSR_DLSS.dll"))
	return nil
}

func ConfigureINI(gameDir string, o InstallOptions, gpu GPU) error {
	ini := filepath.Join(gameDir, "OptiScaler.ini")
	if _, err := os.Stat(ini); err != nil {
		return fmt.Errorf("OptiScaler.ini missing; the release tree was not copied")
	}

	dx11, dx12, vulkan := "fsr22", "xess", "fsr21"
	switch gpu.Architecture {
	case "rtx":
		dx11, dx12, vulkan = "dlss", "dlss", "dlss"
	case "gtx", "intel-arc":
		dx11, dx12, vulkan = "xess", "xess", "fsr21"
	case "rdna4", "rdna1-3":
		dx11, dx12, vulkan = "fsr22_12", "fsr31", "fsr31"
	}
	if o.Upscaler == "fsr" {
		dx11, dx12, vulkan = "fsr22_12", "fsr31", "fsr31"
	} else if o.Upscaler == "xess" {
		dx11, dx12, vulkan = "xess", "xess", "fsr21"
	} else if o.Upscaler == "dlss" {
		dx11, dx12, vulkan = "dlss", "dlss", "dlss"
	}
	for _, kv := range [][2]string{
		{"Dx11Upscaler", dx11}, {"Dx12Upscaler", dx12}, {"VulkanUpscaler", vulkan},
	} {
		if err := IniSet(ini, "Upscalers", kv[0], kv[1]); err != nil {
			return err
		}
	}

	if gpu.Vendor == "NVIDIA" || gpu.Vendor == "Unknown" {
	} else if o.SpoofDLSS {
		if err := IniSet(ini, "Spoofing", "Dxgi", "auto"); err != nil {
			return err
		}
	} else {
		if err := IniSet(ini, "Spoofing", "Dxgi", "false"); err != nil {
			return err
		}
	}

	if o.FrameGen {
		fgIn := orStr(o.FGInput, "fsrfg")
		if err := IniSet(ini, "FrameGen", "Enabled", "true"); err != nil {
			return err
		}
		if err := IniSet(ini, "FrameGen", "FGInput", fgIn); err != nil {
			return err
		}
		if o.FGOutput != "" {
			if err := IniSet(ini, "FrameGen", "FGOutput", o.FGOutput); err != nil {
				return err
			}
		}
	}
	return nil
}

func defaultSpoof(gpu GPU) bool {
	return gpu.Vendor != "NVIDIA"
}

func orStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func GameDirFor(installPath, exePath string) string {
	if exePath != "" {
		if abs, err := filepath.Abs(exePath); err == nil {
			return filepath.Dir(abs)
		}
	}
	return installPath
}

func LocalVersions(storeDir string) []string {
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}
