package opti

import (
	"os"
	"os/exec"
	"sort"
	"strings"
)

func DetectGPUs() []GPU {
	gpus := detectViaLspci()
	if len(gpus) == 0 {
		gpus = detectViaSysfs()
	}
	if len(gpus) == 0 {
		return nil
	}
	primary := -1
	for i, g := range gpus {
		if g.Architecture != "unknown" && g.Architecture != "other-intel" {
			primary = i
			break
		}
	}
	if primary < 0 {
		primary = 0
	}
	for i := range gpus {
		gpus[i].Primary = i == primary
	}
	sort.SliceStable(gpus, func(i, j int) bool { return gpus[i].Primary })
	return gpus
}

func detectViaLspci() []GPU {
	bin, err := exec.LookPath("lspci")
	if err != nil {
		return nil
	}
	out, err := exec.Command(bin, "-mm").Output()
	if err != nil {
		out, err = exec.Command(bin).Output()
		if err != nil {
			return nil
		}
	}
	var gpus []GPU
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "VGA compatible controller") &&
			!strings.Contains(line, "3D controller") &&
			!strings.Contains(line, "Display controller") {
			continue
		}
		name := gpuNameFromLine(line)
		if name == "" {
			continue
		}
		gpus = append(gpus, GPU{
			Name:         name,
			Vendor:       vendorOf(name),
			Architecture: archOf(name),
		})
	}
	return gpus
}

func gpuNameFromLine(line string) string {
	name := line
	if i := strings.Index(name, "controller:"); i >= 0 {
		name = name[i+len("controller:"):]
	} else if i := strings.Index(name, ": "); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.Index(name, "(rev"); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	open := strings.LastIndex(name, "[")
	close := strings.LastIndex(name, "]")
	if open >= 0 && close > open {
		if inner := strings.TrimSpace(name[open+1 : close]); inner != "" {
			return inner
		}
	}
	return name
}

func detectViaSysfs() []GPU {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}
	var gpus []GPU
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		dev := "/sys/class/drm/" + name + "/device"
		vendor := strings.TrimSpace(readFileOr(dev+"/vendor", ""))
		device := strings.TrimSpace(readFileOr(dev+"/device", ""))
		switch vendor {
		case "0x10de", "0x1002", "0x8086":
		default:
			continue
		}
		g := GPU{Name: vendorName(vendor) + " GPU " + device}
		g.Vendor = vendorName(vendor)
		g.Architecture = "unknown"
		gpus = append(gpus, g)
	}
	return gpus
}

func readFileOr(path, def string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	return string(b)
}

func vendorName(hex string) string {
	switch hex {
	case "0x10de":
		return "NVIDIA"
	case "0x1002":
		return "AMD"
	case "0x8086":
		return "Intel"
	}
	return "Unknown"
}

func vendorOf(name string) string {
	u := strings.ToUpper(name)
	switch {
	case strings.Contains(u, "NVIDIA") || strings.Contains(u, "GEFORCE") || strings.Contains(u, "RTX") || strings.Contains(u, "GTX"):
		return "NVIDIA"
	case strings.Contains(u, "AMD") || strings.Contains(u, "RADEON") || strings.Contains(u, "ATI") || strings.Contains(u, "NAVI"):
		return "AMD"
	case strings.Contains(u, "INTEL") || strings.Contains(u, "ARC") || strings.Contains(u, "IRIS") || strings.Contains(u, "UHD"):
		return "Intel"
	}
	return "Unknown"
}

func archOf(name string) string {
	u := strings.ToUpper(name)
	switch vendorOf(name) {
	case "NVIDIA":
		switch {
		case strings.Contains(u, "RTX "):
			return "rtx"
		case strings.Contains(u, "GTX "):
			return "gtx"
		}
		return "other-nvidia"
	case "AMD":
		if strings.Contains(u, "RX 90") || strings.Contains(u, "RX 9") && amdRxNumber(u) >= 9000 {
			return "rdna4"
		}
		if n := amdRxNumber(u); n >= 5000 {
			return "rdna1-3"
		}
		return "other-amd"
	case "Intel":
		if strings.Contains(u, "ARC") || strings.Contains(u, "B5") || strings.Contains(u, "A3") || strings.Contains(u, "A5") || strings.Contains(u, "A7") {
			return "intel-arc"
		}
		return "other-intel"
	}
	return "unknown"
}

func amdRxNumber(u string) int {
	i := strings.Index(u, "RX ")
	if i < 0 {
		return 0
	}
	rest := u[i+3:]
	n := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
		if n > 9999 {
			return 0
		}
	}
	return n
}
