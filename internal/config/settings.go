package config

import (
	"bytes"

	"fmt"
	"hikari/internal/atomicfile"
	"os"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	Engines  EnginesSettings  `toml:"engines"`
	Library  LibrarySettings  `toml:"library"`
	Defaults DefaultsSettings `toml:"defaults"`
	Launch   LaunchSettings   `toml:"launch"`
	Mods     ModsSettings     `toml:"mods"`
}

type ModsSettings struct {
	SourceDir string `toml:"source_dir"`
	Python    string `toml:"python"`
	Filegraph string `toml:"filegraph_extension"`
}

type LaunchSettings struct {
	Comet bool `toml:"comet"`
}

type EnginesSettings struct {
	LegendaryBin string `toml:"legendary_bin"`
	GogdlBin     string `toml:"gogdl_bin"`
	NileBin      string `toml:"nile_bin"`
	CometBin     string `toml:"comet_bin"`
	UmuBin       string `toml:"umu_bin"`
	WineBin      string `toml:"wine_bin"`
}

type LibrarySettings struct {
	HeroicConfig   string `toml:"heroic_config"`
	ImportHeroic   bool   `toml:"import_heroic"`
	SteamGridDBKey string `toml:"steamgriddb_key"`
}

type DefaultsSettings struct {
	WinePrefix  string      `toml:"wine_prefix"`
	WineVersion WineVersion `toml:"wine_version"`
}

func defaultSettings() Settings {
	return Settings{
		Library: LibrarySettings{ImportHeroic: true},
		Launch:  LaunchSettings{Comet: true},
	}
}

func LoadSettings(paths Paths) (Settings, error) {
	s := defaultSettings()
	b, err := os.ReadFile(paths.Settings)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, fmt.Errorf("hikari: cannot read %s: %w", paths.Settings, err)
	}
	if err := toml.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("hikari: %s is not valid TOML: %w", paths.Settings, err)
	}
	return s, nil
}

func SaveSettings(paths Paths, s Settings) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(s); err != nil {
		return err
	}
	return atomicfile.Write(paths.Settings, buf.Bytes())
}
