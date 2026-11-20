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
	nl := iniNewline(b)
	lines := strings.Split(iniBody(b), "\n")
	secAt := -1
	for i, l := range lines {
		if name, ok := iniSectionName(l); ok && strings.EqualFold(name, section) {
			secAt = i
			break
		}
	}
	if secAt < 0 {
		return fmt.Errorf("OptiScaler.ini has no [%s] section", section)
	}

	end := len(lines)
	for i := secAt + 1; i < len(lines); i++ {
		if _, ok := iniSectionName(lines[i]); ok {
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
			return iniWrite(path, lines, nl)
		}
	}
	insert := end
	for insert > secAt+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	out := append([]string{}, lines[:insert]...)
	out = append(out, key+"="+value)
	out = append(out, lines[insert:]...)
	return iniWrite(path, out, nl)
}

// iniSectionName reports the name of a section header line. A trailing comment
// or whitespace after the closing bracket ("[Upscalers] ; pick one") is
// accepted, so the rest of the line does not hide the section.
func iniSectionName(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "[") {
		return "", false
	}
	i := strings.IndexByte(t, ']')
	if i < 0 {
		return "", false
	}
	return strings.TrimSpace(t[1:i]), true
}

// iniBody strips a leading UTF-8 BOM and normalises CRLF line endings so the
// parser only has to deal with plain lines.
func iniBody(b []byte) string {
	s := strings.TrimPrefix(string(b), "\ufeff")
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// iniNewline returns the line ending the file predominantly uses, so rewriting
// it does not mix CRLF and LF.
func iniNewline(b []byte) string {
	crlf := bytes.Count(b, []byte("\r\n"))
	if lf := bytes.Count(b, []byte("\n")) - crlf; crlf > lf {
		return "\r\n"
	}
	return "\n"
}

func iniWrite(path string, lines []string, nl string) error {
	data := []byte(strings.Join(lines, nl))
	if !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, nl...)
	}
	return atomicWrite(path, data)
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
	for _, l := range strings.Split(iniBody(b), "\n") {
		if name, ok := iniSectionName(l); ok {
			inSection = strings.EqualFold(name, section)
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
