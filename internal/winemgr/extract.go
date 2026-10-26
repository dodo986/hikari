package winemgr

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type sha512Hasher struct{ h hash.Hash }

func newSHA512() *sha512Hasher { return &sha512Hasher{h: sha512.New()} }

func (s *sha512Hasher) Write(b []byte) (int, error) { return s.h.Write(b) }

func (s *sha512Hasher) hex() string { return hex.EncodeToString(s.h.Sum(nil)) }

func extractStrip1(ctx context.Context, archive, dest string) error {
	if tarBin, err := exec.LookPath("tar"); err == nil {
		cmd := exec.CommandContext(ctx, tarBin, "-xf", archive, "-C", dest, "--strip-components=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			tail := string(out)
			if len(tail) > 300 {
				tail = tail[len(tail)-300:]
			}
			return fmt.Errorf("extracting %s: %s", filepath.Base(archive), strings.TrimSpace(tail))
		}
		return nil
	}
	if !strings.HasSuffix(archive, ".tar.gz") {
		return fmt.Errorf("tar not found and %s is not .tar.gz", filepath.Base(archive))
	}
	return extractTgzStrip1(archive, dest)
}

func extractTgzStrip1(archive, dest string) error {
	f, err := os.Open(archive)
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
		name := hdr.Name
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.Clean("/"+name))
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
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			_ = os.Chmod(target, os.FileMode(hdr.Mode)&0o777)
		}
	}
}
