package art

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x40, A: 0xff})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func TestScaledFitsTheRequestedWidth(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "cover.jpg")
	writeJPEG(t, source, 600, 900)
	cache := New(dir)

	scaled, err := cache.Scaled(source, 200)
	if err != nil {
		t.Fatalf("scale: %v", err)
	}
	if scaled == source {
		t.Fatal("a 600px wide source should be scaled for a 200px request")
	}
	f, err := os.Open(scaled)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Dx(); got != 200 {
		t.Fatalf("width = %d, want 200", got)
	}
	if got := img.Bounds().Dy(); got != 300 {
		t.Fatalf("height = %d, want 300, the aspect ratio must hold", got)
	}

	again, err := cache.Scaled(source, 200)
	if err != nil || again != scaled {
		t.Fatalf("second call = %q (%v), want the cached copy", again, err)
	}
}

func TestScaledKeepsSmallImages(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "small.jpg")
	writeJPEG(t, source, 120, 180)
	cache := New(dir)
	scaled, err := cache.Scaled(source, 240)
	if err != nil {
		t.Fatalf("scale: %v", err)
	}
	if scaled != source {
		t.Fatalf("scaled = %q, want the original for a source that is already narrow", scaled)
	}
}

func TestScaledRejectsSillyWidths(t *testing.T) {
	cache := New(t.TempDir())
	if _, err := cache.Scaled("x.jpg", 0); err == nil {
		t.Fatal("width 0 should be rejected")
	}
	if _, err := cache.Scaled("x.jpg", 5000); err == nil {
		t.Fatal("width 5000 should be rejected")
	}
}
