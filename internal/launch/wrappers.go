package launch

import (
	"os/exec"
	"strings"

	"hikari/internal/config"
)

func Wrappers(s config.GameSettings, umuBin string) []string {
	var wrappers []string
	if s.Gamescope != nil && (s.Gamescope.EnableLimiter || s.Gamescope.EnableUpscaling) {
		if argv, ok := gamescopeArgs(s.Gamescope); ok {
			wrappers = append(wrappers, argv...)
			wrappers = append(wrappers, "--")
		}
	}
	for _, w := range s.Wrappers {
		wrappers = append(wrappers, w.Exe)
		if w.Args != "" {
			wrappers = append(wrappers, strings.Fields(w.Args)...)
		}
	}
	usingGamescope := s.Gamescope != nil && (s.Gamescope.EnableLimiter || s.Gamescope.EnableUpscaling)
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
		argv = append(argv, strings.Fields(g.AdditionalOptions)...)
	}
	return argv, true
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
