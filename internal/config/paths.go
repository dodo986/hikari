package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	Home         string
	ConfigDir    string
	DataDir      string
	Settings     string
	State        string
	Log          string
	EngineEpic   string
	EngineGOG    string
	EngineAmazon string

	HeroicConfig string
}

func Resolve() Paths {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(os.Getenv("HOME"), ".config")
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return ResolveFrom(cfg, data)
}

func ResolveFrom(configRoot, dataRoot string) Paths {
	home, _ := os.UserHomeDir()
	cfgDir := filepath.Join(configRoot, "hikari")
	dataDir := filepath.Join(dataRoot, "hikari")
	engines := filepath.Join(cfgDir, "engines")
	return Paths{
		Home:         home,
		ConfigDir:    cfgDir,
		DataDir:      dataDir,
		Settings:     filepath.Join(cfgDir, "settings.toml"),
		State:        filepath.Join(dataDir, "state.json"),
		Log:          filepath.Join(dataDir, "hikarid.log"),
		EngineEpic:   filepath.Join(engines, "legendary"),
		EngineGOG:    filepath.Join(engines, "gogdl"),
		EngineAmazon: filepath.Join(engines, "nile"),
		HeroicConfig: filepath.Join(configRoot, "heroic"),
	}
}

func RuntimeDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "hikari")
}

func SocketPath() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "hikari.sock")
}

// WSEndpointPath holds the websocket address and token for local UI clients.
// It is readable by this user only and rewritten on every daemon start.
func WSEndpointPath() string {
	return filepath.Join(RuntimeDir(), "ws.json")
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.ConfigDir, p.DataDir, p.EngineEpic, p.EngineGOG, p.EngineAmazon} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
