package opti

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestIniSetWithBOMHeader: a leading UTF-8 BOM must not hide the first section.
func TestIniSetWithBOMHeader(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "OptiScaler.ini")
	if err := os.WriteFile(ini, []byte("\ufeff[Upscalers]\nDx11Upscaler=auto\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := IniSet(ini, "Upscalers", "Dx11Upscaler", "fsr22_12"); err != nil {
		t.Fatalf("BOM must not hide the section: %v", err)
	}
	if got := IniGet(ini, "Upscalers", "Dx11Upscaler"); got != "fsr22_12" {
		t.Fatalf("read back %q", got)
	}
}

// TestIniSetWithCommentedHeader: a trailing comment after the closing bracket
// must not hide the section either.
func TestIniSetWithCommentedHeader(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "OptiScaler.ini")
	if err := os.WriteFile(ini, []byte("[Upscalers] ; upscalers\nDx12Upscaler=auto\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := IniSet(ini, "Upscalers", "Dx12Upscaler", "fsr31"); err != nil {
		t.Fatalf("commented header must not hide the section: %v", err)
	}
	if got := IniGet(ini, "Upscalers", "Dx12Upscaler"); got != "fsr31" {
		t.Fatalf("read back %q", got)
	}
}

// TestIniSetPreservesCRLF: rewriting one line or inserting a key must keep the
// file's CRLF endings throughout, not mix in bare LF ones.
func TestIniSetPreservesCRLF(t *testing.T) {
	ini := filepath.Join(t.TempDir(), "OptiScaler.ini")
	content := "[Upscalers]\r\nDx11Upscaler=auto\r\nDx12Upscaler=auto\r\n\r\n[FrameGen]\r\nEnabled=auto\r\n"
	if err := os.WriteFile(ini, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := IniSet(ini, "Upscalers", "Dx11Upscaler", "fsr22_12"); err != nil {
		t.Fatal(err)
	}
	if err := IniSet(ini, "FrameGen", "FGInput", "fsrfg"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(ini)
	if err != nil {
		t.Fatal(err)
	}
	if lf, crlf := bytes.Count(b, []byte("\n")), bytes.Count(b, []byte("\r\n")); lf != crlf {
		t.Fatalf("mixed line endings: %d LF vs %d CRLF:\n%q", lf, crlf, b)
	}
	if !bytes.Contains(b, []byte("FGInput=fsrfg\r\n")) {
		t.Fatalf("inserted line must use CRLF:\n%q", b)
	}
	if got := IniGet(ini, "Upscalers", "Dx11Upscaler"); got != "fsr22_12" {
		t.Fatalf("read back %q", got)
	}
}
