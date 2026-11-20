package dxvk

import (
	"context"
	"encoding/binary"
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
	st, err := os.Stat(filepath.Join(prefix, "drive_c", "windows", "system32", "d3d11.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o644 {
		t.Fatalf("installed DLL mode = %04o, want 0644", perm)
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

// writePE writes a minimal synthetic PE image carrying machine as its COFF
// machine field. Only the bytes peMachine reads are populated.
func writePE(t *testing.T, path string, machine uint16) {
	t.Helper()
	const peOff = 0x40
	buf := make([]byte, peOff+6)
	buf[0], buf[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(buf[0x3c:], peOff)
	copy(buf[peOff:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(buf[peOff+4:], machine)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func system32Path(prefix string) string {
	return filepath.Join(prefix, "drive_c", "windows", "system32")
}

func installTree(t *testing.T, prefix, want string) {
	t.Helper()
	tree := writeTree(t, []string{"d3d11.dll"}, []string{"d3d11.dll"})
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	if _, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", tree, nil); err != nil {
		t.Fatal(err)
	}
	dll, err := os.ReadFile(filepath.Join(system32Path(prefix), "d3d11.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if string(dll) != want {
		t.Fatalf("system32 d3d11 came from %q, want the %s tree", dll, want)
	}
}

// TestInstallIntoPrefixWithoutSyswow64UsesPEArch covers a Wine/Proton build
// without 32-bit support: a 64-bit-only prefix has no syswow64, so the
// architecture must come from the system32 DLLs, not from syswow64's absence.
func TestInstallIntoPrefixWithoutSyswow64UsesPEArch(t *testing.T) {
	prefix := prefixDirs(t, false)
	writePE(t, filepath.Join(system32Path(prefix), "kernel32.dll"), 0x8664)
	installTree(t, prefix, "x64")
}

// TestInstallInto32BitPrefixWithPEEvidence keeps the long-standing 32-bit
// behaviour when system32 itself carries i386 DLLs and there is no syswow64.
func TestInstallInto32BitPrefixWithPEEvidence(t *testing.T) {
	prefix := prefixDirs(t, false)
	writePE(t, filepath.Join(system32Path(prefix), "kernel32.dll"), 0x014c)
	installTree(t, prefix, "x32")
}

// TestInstallIntoSniffsOtherSystem32DLL covers the fallback past kernel32.dll:
// when that file carries no usable PE header, any other system32 DLL decides
// the architecture.
func TestInstallIntoSniffsOtherSystem32DLL(t *testing.T) {
	prefix := prefixDirs(t, false)
	system32 := system32Path(prefix)
	if err := os.WriteFile(filepath.Join(system32, "kernel32.dll"), []byte("not a PE"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePE(t, filepath.Join(system32, "ntdll.dll"), 0x8664)
	installTree(t, prefix, "x64")
}

// TestInstallIntoUnreadableSystem32FallsBack pins the fallback: when no PE
// machine can be read, the installer keeps copying the 32-bit tree into
// system32 and does not fail.
func TestInstallIntoUnreadableSystem32FallsBack(t *testing.T) {
	prefix := prefixDirs(t, false)
	if err := os.WriteFile(filepath.Join(system32Path(prefix), "kernel32.dll"), []byte("not a PE"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := writeTree(t, []string{"d3d11.dll"}, []string{"d3d11.dll"})
	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	if _, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", tree, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(system32Path(prefix), "d3d11.dll"))
	if err != nil || string(b) != "x32" {
		t.Fatalf("system32 d3d11 came from %q, want the 32-bit tree", b)
	}
}

// TestInstallIntoKeepsDLLWhenCopyFails guards against a failed copy truncating
// an already-good DLL in the prefix: copyFile writes a temp file and renames it
// over the destination only after the whole copy succeeded.
func TestInstallIntoKeepsDLLWhenCopyFails(t *testing.T) {
	prefix := prefixDirs(t, false)
	system32 := filepath.Join(prefix, "drive_c", "windows", "system32")
	existing := filepath.Join(system32, "d3d11.dll")
	const good = "GOOD"
	if err := os.WriteFile(existing, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "tool-v1")
	x64 := filepath.Join(root, "x64")
	if err := os.MkdirAll(x64, 0o755); err != nil {
		t.Fatal(err)
	}
	// /proc/self/mem opens fine but fails on read, so io.Copy fails midway
	// through the copy with the destination already opened.
	if err := os.Symlink("/proc/self/mem", filepath.Join(x64, "d3d11.dll")); err != nil {
		t.Skipf("cannot create the /proc/self/mem symlink: %v", err)
	}

	i := NewInstaller(t.TempDir(), slog.New(slog.DiscardHandler))
	if _, err := i.InstallInto(context.Background(), prefix, DXVK, "v1", root, nil); err == nil {
		t.Skip("copying /proc/self/mem unexpectedly succeeded on this host")
	}

	b, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("pre-existing DLL vanished after a failed install: %v", err)
	}
	if string(b) != good {
		t.Fatalf("destination DLL = %q after a failed copy, want %q (it was truncated)", b, good)
	}
	entries, err := os.ReadDir(system32)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "d3d11.dll" {
			t.Fatalf("failed copy left %q behind in system32", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(prefix, "current_dxvk")); !os.IsNotExist(err) {
		t.Fatalf("a failed install must not write the version marker: %v", err)
	}
}
