package art

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
)

// Scaled returns a width-bounded copy of a cached image, reusing an earlier
// copy when one exists. A grid cell shows a cover at a few hundred pixels, so
// handing the view a 600x900 original costs decode memory for nothing.
func (c *Cache) Scaled(path string, width int) (string, error) {
	if width < 32 || width > 2048 {
		return "", fmt.Errorf("width must be between 32 and 2048")
	}
	target := scaledPath(path, width)
	if decodable(target) {
		return target, nil
	}
	source, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer source.Close()
	img, format, err := image.Decode(source)
	if err != nil {
		return "", err
	}
	if format != "jpeg" && format != "png" {
		return "", fmt.Errorf("%s is a %s image", filepath.Base(path), format)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= width {
		return path, nil
	}
	height := bounds.Dy() * width / bounds.Dx()
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, bounds, draw.Over, nil)

	if err := writeFileAtomic(target, func(w io.Writer) error {
		return jpeg.Encode(w, scaled, &jpeg.Options{Quality: 82})
	}); err != nil {
		return "", err
	}
	return target, nil
}

// decodable reports whether path already holds a complete image. A leftover
// from a failed or interrupted encode would otherwise be reused forever.
func decodable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	_, _, err = image.DecodeConfig(f)
	return err == nil
}

func scaledPath(path string, width int) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return filepath.Join(filepath.Dir(path), base+"-w"+strconv.Itoa(width)+".jpg")
}
