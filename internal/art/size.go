package art

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Dimensions reads the pixel size of a cached image so the daemon can tell a
// portrait grid from a wide banner instead of trusting a file name.
func Dimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	head := make([]byte, 32)
	n, _ := f.Read(head)
	head = head[:n]
	switch {
	case n >= 24 && string(head[1:4]) == "PNG":
		return int(binary.BigEndian.Uint32(head[16:20])), int(binary.BigEndian.Uint32(head[20:24])), nil
	case n >= 4 && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return webpSize(path)
	case n >= 2 && head[0] == 0xFF && head[1] == 0xD8:
		return jpegSize(f)
	}
	return 0, 0, fmt.Errorf("%s is not a png, jpeg or webp image", path)
}

func jpegSize(f *os.File) (int, int, error) {
	if _, err := f.Seek(2, 0); err != nil {
		return 0, 0, err
	}
	for {
		var marker [2]byte
		if _, err := f.Read(marker[:1]); err != nil {
			return 0, 0, fmt.Errorf("jpeg without a frame header")
		}
		if marker[0] != 0xFF {
			continue
		}
		if _, err := f.Read(marker[1:]); err != nil {
			return 0, 0, fmt.Errorf("jpeg without a frame header")
		}
		if marker[1] == 0xD8 || marker[1] == 0x01 || (marker[1] >= 0xD0 && marker[1] <= 0xD7) {
			continue
		}
		var lenBuf [2]byte
		if _, err := f.Read(lenBuf[:]); err != nil {
			return 0, 0, fmt.Errorf("jpeg without a frame header")
		}
		length := int(binary.BigEndian.Uint16(lenBuf[:]))
		if marker[1] >= 0xC0 && marker[1] <= 0xCF && marker[1] != 0xC4 && marker[1] != 0xC8 && marker[1] != 0xCC {
			frame := make([]byte, 5)
			if _, err := f.Read(frame); err != nil {
				return 0, 0, fmt.Errorf("jpeg frame is truncated")
			}
			h := int(binary.BigEndian.Uint16(frame[1:3]))
			w := int(binary.BigEndian.Uint16(frame[3:5]))
			return w, h, nil
		}
		if length < 2 {
			return 0, 0, fmt.Errorf("jpeg segment has no length")
		}
		if _, err := f.Seek(int64(length-2), 1); err != nil {
			return 0, 0, fmt.Errorf("jpeg without a frame header")
		}
	}
}

func webpSize(path string) (int, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	if len(b) < 30 {
		return 0, 0, fmt.Errorf("webp is truncated")
	}
	switch string(b[12:16]) {
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, nil
	case "VP8 ":
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3FFF), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3FFF), nil
	case "VP8L":
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3FFF) + 1, int((bits>>14)&0x3FFF) + 1, nil
	}
	return 0, 0, fmt.Errorf("unknown webp flavour")
}
