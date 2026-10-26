package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestLoneTar(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 512)
	copy(data[257:262], "ustar")
	if err := os.WriteFile(filepath.Join(dir, "eac_runtime"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loneTar(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "eac_runtime") {
		t.Fatalf("loneTar = %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := loneTar(dir); got != "" {
		t.Fatalf("two files must not look like an inner tar: %q", got)
	}
}

func TestMergeReplacesSameName(t *testing.T) {
	stage := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "eac_runtime"), []byte("tar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stage, "eac_runtime", "v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := merge(stage, dest); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dest, "eac_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir() {
		t.Fatal("the tar file was not replaced by the extracted directory")
	}
}

func tgz(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractTgzKeepsPathsInside(t *testing.T) {
	file := tgz(t, map[string]string{
		"hikarid":          "bin",
		"quickshell/a.qml": "qml",
		"../escape":        "nope",
	})
	dest := t.TempDir()
	if err := ExtractTgz(file, dest); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"hikarid", "quickshell/a.qml", "escape"} {
		if _, err := os.Stat(filepath.Join(dest, p)); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape")); err == nil {
		t.Fatal("archive escaped its destination")
	}
}
