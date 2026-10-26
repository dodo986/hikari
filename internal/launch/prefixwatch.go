package launch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const environLimit = 1 << 20

func PrefixProcesses(prefix string) []int {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return nil
	}
	want := map[string]bool{
		prefix:                       true,
		filepath.Join(prefix, "pfx"): true,
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", e.Name(), "environ"))
		if err != nil {
			continue
		}
		if len(env) > environLimit {
			continue
		}
		if envMatchesPrefix(env, want) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func envMatchesPrefix(env []byte, want map[string]bool) bool {
	for _, kv := range strings.Split(string(env), "\x00") {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if key != "WINEPREFIX" && key != "STEAM_COMPAT_DATA_PATH" {
			continue
		}
		if want[strings.TrimSuffix(val, "/")] {
			return true
		}
	}
	return false
}
