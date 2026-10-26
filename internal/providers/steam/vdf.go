package steam

import (
	"fmt"
	"strings"
)

func parseVDF(text string) (map[string]any, error) {
	p := &vdfParser{src: text}
	p.skipSpace()
	if p.eat('{') {
		m, err := p.parseMap()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos != len(p.src) {
			return nil, fmt.Errorf("vdf: trailing content at %d", p.pos)
		}
		return m, nil
	}
	key, err := p.parseToken()
	if err != nil {
		return nil, fmt.Errorf("vdf: expected '{' or root key: %w", err)
	}
	if !p.eat('{') {
		return nil, fmt.Errorf("vdf: root key %q not followed by '{{'", key)
	}
	m, err := p.parseMap()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("vdf: trailing content at %d", p.pos)
	}
	return map[string]any{key: m}, nil
}

type vdfParser struct {
	src string
	pos int
}

func (p *vdfParser) skipSpace() {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; c {
		case ' ', '\t', '\r', '\n':
			p.pos++
		case '/', '-':
			if c == '#' || strings.HasPrefix(p.src[p.pos:], "//") {
				for p.pos < len(p.src) && p.src[p.pos] != '\n' {
					p.pos++
				}
				continue
			}
			return
		default:
			return
		}
	}
}

func (p *vdfParser) eat(want byte) bool {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == want {
		p.pos++
		return true
	}
	return false
}

func (p *vdfParser) parseMap() (map[string]any, error) {
	out := map[string]any{}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("vdf: unexpected end of file")
		}
		if p.src[p.pos] == '}' {
			p.pos++
			return out, nil
		}
		key, err := p.parseToken()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("vdf: key %q without value", key)
		}
		switch p.src[p.pos] {
		case '{':
			p.pos++
			sub, err := p.parseMap()
			if err != nil {
				return nil, err
			}
			out[key] = sub
		case '"':
			val, err := p.parseToken()
			if err != nil {
				return nil, err
			}
			out[key] = val
		default:
			start := p.pos
			for p.pos < len(p.src) {
				c := p.src[p.pos]
				if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '{' || c == '}' {
					break
				}
				p.pos++
			}
			out[key] = p.src[start:p.pos]
		}
	}
}

func (p *vdfParser) parseToken() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != '"' {
		return "", fmt.Errorf("vdf: expected quoted token at %d", p.pos)
	}
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case '\\':
			p.pos++
			if p.pos < len(p.src) {
				b.WriteByte(p.src[p.pos])
				p.pos++
			}
		case '"':
			p.pos++
			return b.String(), nil
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return "", fmt.Errorf("vdf: unterminated string")
}

func vdfString(m map[string]any, path ...string) string {
	cur := m
	for i, k := range path {
		v, ok := cur[k]
		if !ok {
			return ""
		}
		if i == len(path)-1 {
			s, _ := v.(string)
			return s
		}
		cur, ok = v.(map[string]any)
		if !ok {
			return ""
		}
	}
	return ""
}

func vdfMaps(m map[string]any, key string) []map[string]any {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		var out []map[string]any
		for _, child := range t {
			if cm, ok := child.(map[string]any); ok {
				out = append(out, cm)
			}
		}
		return out
	case []any:
		var out []map[string]any
		for _, child := range t {
			if cm, ok := child.(map[string]any); ok {
				out = append(out, cm)
			}
		}
		return out
	}
	return nil
}
