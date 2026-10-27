package mods

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// StopNote explains a launch that ended early: which loader wrote the last log
// of that launch and, when the log stops in the middle of a plugin, which one.
type StopNote struct {
	Loader  string `json:"loader,omitempty"`
	Plugin  string `json:"plugin,omitempty"`
	Log     string `json:"log,omitempty"`
	Line    string `json:"line,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// LoaderLog is one log a launch wrote, as the backend read it.
type LoaderLog struct {
	Loader string   `json:"loader"`
	Path   string   `json:"path"`
	Lines  []string `json:"lines"`
}

// loadingPlugin matches the line a loader writes before it runs a plugin, which
// is the last thing on disk when that plugin takes the game down with it. The
// extension list keeps a launch of the game's own executable out of it.
var loadingPlugin = regexp.MustCompile(`(?i)loading plugin from ["']([^"']+\.(?:dll|asi|esp|esm|esl|pak|bsa|ba2))["']`)

// settledPlugin says the loader got past that line on its own.
var settledPlugin = regexp.MustCompile(`(?i)has been loaded|has failed`)

const noteLineWidth = 120

// launchSlack lets a log written the moment the game died count as that launch.
const launchSlack = 2 * time.Second

// StopNoteFrom turns the logs of one launch into the one sentence worth showing
// a player. It answers with nil when no loader wrote anything, because then
// there is nothing to say about mods.
func StopNoteFrom(logs []LoaderLog) *StopNote {
	for _, log := range logs {
		plugin, line := unfinishedPlugin(log.Lines)
		if plugin == "" {
			continue
		}
		return &StopNote{
			Loader:  log.Loader,
			Plugin:  plugin,
			Log:     log.Path,
			Line:    line,
			Message: fmt.Sprintf("The game stopped while %s was loading %s", label(log.Loader), plugin),
			Fix: fmt.Sprintf("Disable %s in Mods and launch again: if the game then starts, "+
				"that plugin is the one to update", plugin),
		}
	}
	for _, log := range logs {
		line := lastNoteLine(log.Lines)
		if line == "" {
			continue
		}
		return &StopNote{
			Loader: log.Loader,
			Log:    log.Path,
			Line:   line,
			Message: fmt.Sprintf("The game stopped and the last thing %s wrote was: %s",
				label(log.Loader), shorten(line, noteLineWidth)),
		}
	}
	return nil
}

// unfinishedPlugin reads the tail of a log: a plugin that is announced and then
// never reported back is the one that was loading when the game went away.
func unfinishedPlugin(lines []string) (string, string) {
	at := -1
	plugin := ""
	for i, line := range lines {
		match := loadingPlugin.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		at, plugin = i, pluginName(match[1])
	}
	if at < 0 {
		return "", ""
	}
	for _, line := range lines[at+1:] {
		if settledPlugin.MatchString(line) {
			return "", ""
		}
	}
	return plugin, strings.TrimSpace(lines[at])
}

func lastNoteLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

func pluginName(path string) string {
	if cut := strings.LastIndexAny(path, `\/`); cut >= 0 {
		path = path[cut+1:]
	}
	if dot := strings.LastIndexByte(path, '.'); dot > 0 {
		path = path[:dot]
	}
	return path
}

// label names a loader inside a sentence, keeping the backend's own wording.
func label(loader string) string {
	if loader == "" {
		return "the game"
	}
	return loader
}

func shorten(line string, width int) string {
	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	return string(runes[:width-1]) + "…"
}
