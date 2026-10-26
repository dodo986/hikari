package launch

import (
	"os"
	"path/filepath"

	"hikari/internal/config"
)

func WineLibs(wineBin string) (lib, lib32 string) {
	if wineBin == "" {
		return "", ""
	}
	dir := filepath.Dir(wineBin)
	if isDir(filepath.Join(dir, "..", "lib")) {
		lib32 = filepath.Clean(filepath.Join(dir, "..", "lib"))
	}
	if isDir(filepath.Join(dir, "..", "lib64")) {
		lib = filepath.Clean(filepath.Join(dir, "..", "lib64"))
	}
	return lib, lib32
}

func SetupWineLibEnv(s config.GameSettings, wineType, wineBin string, env map[string]string) {
	if wineType != WinePlain || s.PreferSystemLibs || wineBin == "" {
		return
	}
	lib, lib32 := WineLibs(wineBin)
	if lib == "" || lib32 == "" {
		return
	}
	if _, ok := env["ORIG_LD_LIBRARY_PATH"]; !ok {
		env["ORIG_LD_LIBRARY_PATH"] = os.Getenv("LD_LIBRARY_PATH")
	}
	env["LD_LIBRARY_PATH"] = joinPaths(lib, lib32, os.Getenv("LD_LIBRARY_PATH"))

	if gst64, gst32 := filepath.Join(lib, "gstreamer-1.0"), filepath.Join(lib32, "gstreamer-1.0"); isDir(gst64) && isDir(gst32) {
		env["GST_PLUGIN_SYSTEM_PATH_1_0"] = gst64 + ":" + gst32
	}
	if wine64, wine32 := filepath.Join(lib, "wine"), filepath.Join(lib32, "wine"); isDir(wine64) && isDir(wine32) {
		env["WINEDLLPATH"] = wine64 + ":" + wine32
	}
}

func joinPaths(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += ":"
		}
		out += p
	}
	return out
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
