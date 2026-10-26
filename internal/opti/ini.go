package opti

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func IniSet(path, section, key, value string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	secAt := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			if strings.EqualFold(strings.Trim(t, "[]"), section) {
				secAt = i
				break
			}
		}
	}
	if secAt < 0 {
		return fmt.Errorf("OptiScaler.ini has no [%s] section", section)
	}

	end := len(lines)
	for i := secAt + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			end = i
			break
		}
	}
	for i := secAt + 1; i < end; i++ {
		l := lines[i]
		if strings.HasPrefix(strings.TrimSpace(l), ";") || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if k, _, ok := splitIniLine(l); ok && strings.EqualFold(k, key) {
			lines[i] = key + "=" + value
			return atomicWrite(path, []byte(strings.Join(lines, "\n")))
		}
	}
	insert := end
	for insert > secAt+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	out := append([]string{}, lines[:insert]...)
	out = append(out, key+"="+value)
	out = append(out, lines[insert:]...)
	return atomicWrite(path, []byte(strings.Join(out, "\n")))
}

func splitIniLine(l string) (key, value string, ok bool) {
	l = strings.TrimSpace(l)
	i := strings.IndexByte(l, '=')
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(l[:i]), strings.TrimSpace(l[i+1:]), true
}

func IniGet(path, section, key string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	inSection := false
	for _, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inSection = strings.EqualFold(strings.Trim(t, "[]"), section)
			continue
		}
		if !inSection {
			continue
		}
		if k, v, ok := splitIniLine(l); ok && strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func atomicWrite(path string, data []byte) error {
	if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	f, err := os.CreateTemp(dirOfPath(path), ".ini-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func dirOfPath(path string) string {
	i := strings.LastIndexByte(path, '/')
	if i <= 0 {
		return "."
	}
	return path[:i]
}
