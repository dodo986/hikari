package art

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 9, G: 9, B: 9, A: 255})
	p := filepath.Join(dir, "a.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDimensionsReadsPNG(t *testing.T) {
	p := writePNG(t, t.TempDir(), 1200, 1800)
	w, h, err := Dimensions(p)
	if err != nil || w != 1200 || h != 1800 {
		t.Fatalf("Dimensions = %d x %d, %v", w, h, err)
	}
}

func TestDimensionsReadsJPEG(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	img.Set(1, 1, color.RGBA{R: 7, G: 7, B: 7, A: 255})
	p := filepath.Join(dir, "a.jpg")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	f.Close()
	w, h, err := Dimensions(p)
	if err != nil || w != 640 || h != 360 {
		t.Fatalf("Dimensions = %d x %d, %v", w, h, err)
	}
}

func TestDimensionsReadsWebPVP8X(t *testing.T) {
	dir := t.TempDir()
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(22))
	b.WriteString("WEBPVP8X")
	binary.Write(&b, binary.LittleEndian, uint32(10))
	b.Write([]byte{0, 0, 0, 0})
	b.Write([]byte{0x2F, 0x04, 0, 0x0F, 0x01, 0})
	p := filepath.Join(dir, "a.webp")
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	w, h, err := Dimensions(p)
	if err != nil || w != 1072 || h != 272 {
		t.Fatalf("Dimensions = %d x %d, %v", w, h, err)
	}
}

func TestDimensionsRefusesWhatItCannotRead(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Dimensions(p); err == nil {
		t.Fatal("plain text must not pass as an image")
	}
}
