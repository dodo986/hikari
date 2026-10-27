package launch

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

// PrefixStart is when a prefix process was started, or the zero time when
// /proc does not say.
func PrefixStart(pid int) time.Time {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return time.Time{}
	}
	// Field 22 counts clock ticks since boot; the command name in field 2 may
	// hold spaces, so count from the closing parenthesis instead.
	close := strings.LastIndexByte(string(stat), ')')
	if close < 0 {
		return time.Time{}
	}
	fields := strings.Fields(string(stat)[close+1:])
	if len(fields) < 20 {
		return time.Time{}
	}
	ticks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return time.Time{}
	}
	boot, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	for _, line := range strings.Split(string(boot), "\n") {
		rest, ok := strings.CutPrefix(line, "btime ")
		if !ok {
			continue
		}
		secs, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(secs, 0).Add(time.Duration(ticks) * time.Second / 100)
	}
	return time.Time{}
}

// PrefixCmdline is a process command line with its arguments joined by spaces.
func PrefixCmdline(pid int) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	return strings.Join(strings.Split(string(raw), "\x00"), " ")
}
