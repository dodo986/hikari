package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"hikari/internal/config"
)

type Engine string

const (
	Legendary Engine = "legendary"
	Gogdl     Engine = "gogdl"
	Nile      Engine = "nile"
	Comet     Engine = "comet"
	Umu       Engine = "umu"
	Wine      Engine = "wine"
)

type EngineStatus struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
}

type Manager struct {
	settings config.EnginesSettings
	runner   *Runner
}

func NewManager(settings config.EnginesSettings, runner *Runner) *Manager {
	return &Manager{settings: settings, runner: runner}
}

func (e Engine) binaryName() string {
	if e == Umu {
		return "umu-run"
	}
	return string(e)
}

func (m *Manager) Resolve(e Engine) (string, bool) {
	var override string
	switch e {
	case Legendary:
		override = m.settings.LegendaryBin
	case Gogdl:
		override = m.settings.GogdlBin
	case Nile:
		override = m.settings.NileBin
	case Comet:
		override = m.settings.CometBin
	case Umu:
		override = m.settings.UmuBin
	case Wine:
		override = m.settings.WineBin
	}
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, true
		}
		return "", false
	}
	path, err := exec.LookPath(e.binaryName())
	if err != nil {
		return "", false
	}
	return path, true
}

func (m *Manager) Version(ctx context.Context, e Engine) string {
	bin, ok := m.Resolve(e)
	if !ok {
		return ""
	}
	vctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := m.runner.Run(vctx, bin, []string{"--version"}, RunOptions{})
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		out = strings.TrimSpace(res.Stderr)
	}
	if i := strings.IndexAny(out, "\r\n"); i >= 0 {
		out = out[:i]
	}
	return out
}

func (m *Manager) Status(ctx context.Context) map[string]EngineStatus {
	out := map[string]EngineStatus{}
	for _, e := range []Engine{Legendary, Gogdl, Nile, Comet, Umu, Wine} {
		st := EngineStatus{}
		if bin, ok := m.Resolve(e); ok {
			st.Available = true
			st.Path = bin
			st.Version = m.Version(ctx, e)
		}
		out[string(e)] = st
	}
	return out
}

func (m *Manager) Require(e Engine) (string, error) {
	bin, ok := m.Resolve(e)
	if !ok {
		setting := ""
		switch e {
		case Legendary:
			setting = "legendary_bin"
		case Gogdl:
			setting = "gogdl_bin"
		case Nile:
			setting = "nile_bin"
		case Comet:
			setting = "comet_bin"
		case Umu:
			setting = "umu_bin"
		case Wine:
			setting = "wine_bin"
		}
		return "", fmt.Errorf("%s not found; install its package or set engines.%s in settings.toml", e, setting)
	}
	return bin, nil
}
