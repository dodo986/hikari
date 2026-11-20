package dxvk

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PE COFF machine types we care about when picking the prefix architecture.
const (
	imageFileMachineI386  = 0x014c
	imageFileMachineIA64  = 0x0200
	imageFileMachineAMD64 = 0x8664
	imageFileMachineARM64 = 0xaa64
)

func Locate(tool Tool, tree string) (x64, x86 string, ok bool) {
	sp, known := specs[tool]
	if !known {
		return "", "", false
	}
	_ = filepath.WalkDir(tree, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(tree, path)
		if rerr != nil || strings.Count(rel, string(os.PathSeparator)) > 2 {
			return filepath.SkipDir
		}
		switch d.Name() {
		case "x64":
			if x64 == "" {
				x64 = path
			}
		default:
			if d.Name() == sp.x86Dir && x86 == "" {
				x86 = path
			}
		}
		return nil
	})
	return x64, x86, x64 != ""
}

func (i *Installer) InstallInto(ctx context.Context, prefix string, tool Tool, version, tree string, reg RegFunc) ([]string, error) {
	x64, x86, ok := Locate(tool, tree)
	if !ok {
		return nil, fmt.Errorf("%s: no x64 directory under %s", tool, tree)
	}
	system32 := filepath.Join(prefix, "drive_c", "windows", "system32")
	syswow64 := filepath.Join(prefix, "drive_c", "windows", "syswow64")
	// A missing syswow64 is not proof of a 32-bit prefix: a Wine/Proton build
	// without 32-bit support makes a 64-bit-only prefix. Fall back to the PE
	// machine type of an existing system32 DLL.
	wow64 := prefixIs64(prefix)

	names := map[string]bool{}
	var installed []string
	if wow64 {
		copied, err := copyDLLs(x64, system32)
		if err != nil {
			return nil, err
		}
		for _, n := range copied {
			names[n] = true
		}
		if x86 != "" {
			if st, err := os.Stat(syswow64); err == nil && st.IsDir() {
				copied, err := copyDLLs(x86, syswow64)
				if err != nil {
					return nil, err
				}
				for _, n := range copied {
					names[n] = true
				}
			}
		}
	} else {
		src := x86
		if src == "" {
			src = x64
		}
		copied, err := copyDLLs(src, system32)
		if err != nil {
			return nil, err
		}
		for _, n := range copied {
			names[n] = true
		}
	}

	for n := range names {
		installed = append(installed, n)
	}
	sort.Strings(installed)
	for _, n := range installed {
		if reg == nil {
			continue
		}

		if err := reg(ctx, strings.TrimSuffix(n, ".dll")); err != nil {
			return nil, fmt.Errorf("register %s: %w", n, err)
		}
	}
	if err := os.WriteFile(filepath.Join(prefix, "current_"+string(tool)), []byte(version), 0o644); err != nil {
		return nil, err
	}
	return installed, nil
}

// prefixIs64 reports whether the prefix holds 64-bit Windows DLLs. A syswow64
// directory is conclusive (wow64 prefix). Otherwise the architecture is sniffed
// from a system32 DLL; if nothing recognisable is there, it reports false so the
// installer keeps its historical 32-bit behaviour.
func prefixIs64(prefix string) bool {
	windows := filepath.Join(prefix, "drive_c", "windows")
	if st, err := os.Stat(filepath.Join(windows, "syswow64")); err == nil && st.IsDir() {
		return true
	}
	system32 := filepath.Join(windows, "system32")
	if m, ok := peMachine(filepath.Join(system32, "kernel32.dll")); ok {
		return machineIs64(m)
	}
	entries, err := os.ReadDir(system32)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || strings.EqualFold(e.Name(), "kernel32.dll") ||
			!strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			continue
		}
		if m, ok := peMachine(filepath.Join(system32, e.Name())); ok {
			return machineIs64(m)
		}
	}
	return false
}

func machineIs64(m uint16) bool {
	switch m {
	case imageFileMachineAMD64, imageFileMachineARM64, imageFileMachineIA64:
		return true
	default:
		return false
	}
}

// peMachine reads the COFF machine field of the PE at path, looking only at the
// first few hundred bytes. It reports false when the file cannot be read or
// does not carry a recognisable MZ/PE header.
func peMachine(path string) (uint16, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return 0, false
	}
	buf = buf[:n]
	if n < 0x40 || buf[0] != 'M' || buf[1] != 'Z' {
		return 0, false
	}
	off := binary.LittleEndian.Uint32(buf[0x3c:0x40])
	if off > uint32(n-6) {
		return 0, false
	}
	if buf[off] != 'P' || buf[off+1] != 'E' || buf[off+2] != 0 || buf[off+3] != 0 {
		return 0, false
	}
	return binary.LittleEndian.Uint16(buf[off+4 : off+6]), true
}

func copyDLLs(src, dst string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".dll") {
			continue
		}
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return nil, err
		}
		names = append(names, e.Name())
	}
	return names, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Write to a temp file and rename it over dst so a failed copy never
	// truncates an already-good DLL in the prefix.
	out, err := os.CreateTemp(dir, ".hikari-dll-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	cleanup := func() {
		_ = out.Close()
		_ = os.Remove(tmp)
	}
	if _, err := io.Copy(out, in); err != nil {
		cleanup()
		return err
	}
	if err := out.Chmod(0o644); err != nil {
		cleanup()
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
