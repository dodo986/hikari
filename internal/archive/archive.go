package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Extract(ctx context.Context, archive, dest string) error {
	bin := ""
	for _, cand := range []string{"7zz", "7z"} {
		if p, err := exec.LookPath(cand); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		return fmt.Errorf("7z not found; install 7zip so release archives can be extracted")
	}
	if err := run(ctx, bin, archive, dest); err != nil {
		return err
	}
	inner, err := loneTar(dest)
	if err != nil || inner == "" {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := run(ctx, bin, inner, stage); err != nil {
		return err
	}

	if err := os.Remove(inner); err != nil {
		return err
	}
	return merge(stage, dest)
}

func merge(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		target := filepath.Join(dst, e.Name())
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(src, e.Name()), target); err != nil {
			return err
		}
	}
	return nil
}

func run(ctx context.Context, bin, archive, dest string) error {
	out, err := exec.CommandContext(ctx, bin, "x", "-y", "-o"+dest, archive).CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 300 {
			tail = tail[len(tail)-300:]
		}
		return fmt.Errorf("7z extraction failed: %s", strings.TrimSpace(tail))
	}
	return nil
}

func loneTar(dest string) (string, error) {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return "", err
	}
	var file string
	for _, e := range entries {
		if e.IsDir() {
			return "", nil
		}
		if file != "" {
			return "", nil
		}
		file = filepath.Join(dest, e.Name())
	}
	if file == "" {
		return "", nil
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	magic := make([]byte, 262)
	if _, err := io.ReadFull(f, magic); err != nil {
		return "", nil
	}
	if string(magic[257:262]) != "ustar" {
		return "", nil
	}
	return file, nil
}

func ExtractTgz(archiveFile, dest string) error {
	f, err := os.Open(archiveFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.Clean("/"+hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			if err := out.Chmod(mode); err != nil {
				out.Close()
				return err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, 512<<20)); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
}
