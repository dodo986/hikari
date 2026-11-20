package launch

import (
	"os/exec"
	"strings"

	"hikari/internal/config"
)

func Wrappers(s config.GameSettings, umuBin string) []string {
	var wrappers []string
	usingGamescope := false
	if s.Gamescope != nil && (s.Gamescope.EnableLimiter || s.Gamescope.EnableUpscaling) {
		if argv, ok := gamescopeArgs(s.Gamescope); ok {
			wrappers = append(wrappers, argv...)
			wrappers = append(wrappers, "--")
			usingGamescope = true
		}
	}
	for _, w := range s.Wrappers {
		wrappers = append(wrappers, w.Exe)
		if w.Args != "" {
			wrappers = append(wrappers, splitArgs(w.Args)...)
		}
	}
	// MangoHud reports its own frame timings and would fight gamescope, so it
	// is only left out when gamescope is really in the chain: a configured but
	// missing gamescope must not silently disable the overlay and the limiter.
	if s.ShowMangoHud && !usingGamescope {
		wrappers = append(wrappers, "mangohud", "--dlsym")
	}
	if s.UseGameMode {
		wrappers = append(wrappers, "gamemoderun")
	}
	if umuBin != "" {
		wrappers = append(wrappers, umuBin)
	}
	return wrappers
}

func gamescopeArgs(g *config.GamescopeSettings) ([]string, bool) {
	bin, err := exec.LookPath("gamescope")
	if err != nil {
		return nil, false
	}
	argv := []string{bin}
	if g.EnableUpscaling {
		if g.GameWidth != "" {
			argv = append(argv, "-w", g.GameWidth)
		}
		if g.GameHeight != "" {
			argv = append(argv, "-h", g.GameHeight)
		}
		if g.UpscaleWidth != "" {
			argv = append(argv, "-W", g.UpscaleWidth)
		}
		if g.UpscaleHeight != "" {
			argv = append(argv, "-H", g.UpscaleHeight)
		}
		switch g.UpscaleMethod {
		case "fsr":
			argv = append(argv, "-F", "fsr")
		case "nis":
			argv = append(argv, "-F", "nis")
		case "integer":
			argv = append(argv, "-S", "integer")
		case "stretch":
			argv = append(argv, "-S", "stretch")
		}
		if g.WindowType == "fullscreen" {
			argv = append(argv, "-f")
		} else if g.WindowType == "borderless" {
			argv = append(argv, "-b")
		}
	}
	if g.EnableLimiter {
		if g.FpsLimiter != "" {
			argv = append(argv, "-r", g.FpsLimiter)
		}
	}
	if g.AdditionalOptions != "" {
		argv = append(argv, splitArgs(g.AdditionalOptions)...)
	}
	return argv, true
}

// splitArgs splits a user supplied argument string the way a shell would:
// unquoted whitespace separates arguments, and single or double quotes group
// them. strings.Fields used to split quoted values, so a wrapper flag like
// --config "/home/me/my dir/cfg.ini" never survived.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inWord := false
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			inWord, escaped = true, false
		case quote != 0:
			switch {
			case r == quote:
				quote = 0
			case r == '\\' && quote == '"':
				escaped = true
			default:
				cur.WriteRune(r)
			}
			inWord = true
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == '\\':
			inWord, escaped = true, true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if escaped {
		cur.WriteRune('\\')
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}

func ShellJoin(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, ` "'\`) {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}
