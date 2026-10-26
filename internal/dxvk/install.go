package dxvk

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	wow64 := false
	if st, err := os.Stat(syswow64); err == nil && st.IsDir() {
		wow64 = true
	}

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
			copied, err := copyDLLs(x86, syswow64)
			if err != nil {
				return nil, err
			}
			for _, n := range copied {
				names[n] = true
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
