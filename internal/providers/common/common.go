package common

import (
	"strconv"
	"strings"

	"hikari/internal/engine"
	"hikari/internal/library"
)

func ProgressFromFrame(state string, f engine.Frame) library.InstallProgress {
	pr := library.InstallProgress{State: state, Bytes: f.Bytes, ETA: f.ETA}
	pr.Percent = f.Percent
	pr.DownSpeed = f.DownSpeed
	pr.DiskSpeed = f.DiskSpeed
	return pr
}

func FloatPtr(v float64) *float64 { return &v }

func HumanSize(bytes float64) string {
	if bytes <= 0 {
		return "0"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	v := bytes
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return strconv.FormatFloat(v, 'f', 0, 64) + " B"
	}
	return strconv.FormatFloat(v, 'f', 2, 64) + " " + units[i]
}

func FirstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	out := strings.Join(lines, "; ")
	if len(out) > 400 {
		out = out[:400] + "..."
	}
	return out
}

func RedactLine(secret string) func(string) string {
	return func(line string) string {
		if secret != "" && strings.Contains(line, secret) {
			return "<redacted>"
		}
		return line
	}
}

func InstallCanceled(progress func(library.InstallProgress)) error {
	if progress != nil {
		progress(library.InstallProgress{State: "canceled"})
	}
	return library.ErrInstallCanceled
}
