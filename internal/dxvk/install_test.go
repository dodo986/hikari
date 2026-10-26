package dxvk

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeTree(t *testing.T, x64DLLs, x86DLLs []string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tool-v1")
	for dir, dlls := range map[string][]string{"x64": x64DLLs, "x32": x86DLLs} {
		if len(dlls) == 0 {
			continue
		}
		p := filepath.Join(root, dir)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, d := range dlls {
			if err := os.WriteFile(filepath.Join(p, d), []byte(dir), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func prefixDirs(t *testing.T, wow64 bool) string {
	t.Helper()
	prefix := t.TempDir()
	dirs := []string{filepath.Join(prefix, "drive_c", "windows", "system32")}
	if wow64 {
		dirs = append(dirs, filepath.Join(prefix, "drive_c", "windows", "syswow64"))
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return prefix
}

func TestInstallIntoWOW64Prefix(t *testing.T) {
	prefix := prefixDirs(t, true)
	tree := writeTree(t, []string{"d3d11.dll", "dxgi.dll"}, []string{"d3d11.dll"})
	var regs []string
	reg := func(_ context.Context, dll string) error {
		regs = append(regs, dll)
		return nil
	}
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	installed, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", tree, reg)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(regs)
	if strings.Join(regs, ",") != "d3d11,dxgi" {
		t.Fatalf("registered %v, want d3d11,dxgi without extension", regs)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %v", installed)
	}
	for _, p := range []string{
		filepath.Join(prefix, "drive_c", "windows", "system32", "d3d11.dll"),
		filepath.Join(prefix, "drive_c", "windows", "syswow64", "d3d11.dll"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(prefix, "current_dxvk"))
	if err != nil || string(b) != "v1" {
		t.Fatalf("version marker = %q, %v", b, err)
	}
}

func TestInstallInto32BitPrefix(t *testing.T) {
	prefix := prefixDirs(t, false)
	tree := writeTree(t, []string{"d3d11.dll"}, []string{"d3d11.dll", "dxgi.dll"})
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	if _, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", tree, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "drive_c", "windows", "system32", "dxgi.dll")); err != nil {
		t.Fatal("32-bit DLL not copied to system32")
	}
	b, err := os.ReadFile(filepath.Join(prefix, "drive_c", "windows", "system32", "d3d11.dll"))
	if err != nil || string(b) != "x32" {
		t.Fatalf("system32 d3d11 came from %q, want the 32-bit tree", b)
	}
}

func TestInstallIntoNeedsX64(t *testing.T) {
	prefix := prefixDirs(t, true)
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	if _, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", t.TempDir(), nil); err == nil {
		t.Fatal("an empty tree must fail")
	}
}
