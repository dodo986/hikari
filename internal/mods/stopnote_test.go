package mods

import "testing"

func TestStopNoteNamesThePluginThatWasLoading(t *testing.T) {
	logs := []LoaderLog{{
		Loader: "RED4ext",
		Path:   "/games/Cyb2077/red4ext/logs/red4ext-1.log",
		Lines: []string{
			"[info] [RED4ext] Loading plugin from 'X:\\Cyb2077\\red4ext\\plugins\\ArchiveXL\\ArchiveXL.dll'...",
			"[info] [RED4ext] ArchiveXL (version: 1.28.0) has been loaded",
			"[info] [RED4ext] Loading plugin from 'X:\\Cyb2077\\red4ext\\plugins\\Codeware\\Codeware.dll'...",
		},
	}}
	note := StopNoteFrom(logs)
	if note == nil || note.Plugin != "Codeware" || note.Loader != "RED4ext" {
		t.Fatalf("got %+v", note)
	}
	if note.Message != "The game stopped while RED4ext was loading Codeware" {
		t.Fatalf("message: %q", note.Message)
	}
}

func TestStopNoteLeavesAFinishedPluginAlone(t *testing.T) {
	logs := []LoaderLog{{
		Loader: "RED4ext",
		Path:   "/games/Cyb2077/red4ext/logs/red4ext-1.log",
		Lines: []string{
			"[info] [RED4ext] Loading plugin from 'X:\\Cyb\\Codeware\\Codeware.dll'...",
			"[info] [RED4ext] Codeware (version: 1.19.0) has been loaded",
			"[info] [RED4ext] RED4ext is starting up...",
		},
	}}
	note := StopNoteFrom(logs)
	if note == nil {
		t.Fatal("expected a note about the last line")
	}
	if note.Plugin != "" || note.Message != "The game stopped and the last thing RED4ext wrote was: [info] [RED4ext] RED4ext is starting up..." {
		t.Fatalf("got %+v", note)
	}
}

func TestStopNoteStaysQuietWithoutLogs(t *testing.T) {
	if note := StopNoteFrom(nil); note != nil {
		t.Fatalf("want no note, got %+v", note)
	}
	if note := StopNoteFrom([]LoaderLog{{Loader: "CET", Path: "/x.log", Lines: []string{" ", ""}}}); note != nil {
		t.Fatalf("want no note for an empty log, got %+v", note)
	}
}

func TestStopNoteReadsLoadersOtherThanRed4ext(t *testing.T) {
	logs := []LoaderLog{{
		Loader: "SKSE",
		Path:   "/games/Skyrim/My Games/SKSE/skse64.log",
		Lines: []string{
			"checking plugin \"SkyUI_SE.esp\"",
			"loading plugin from 'Data/SKSE/Plugins/ConsoleUtilSSE.dll'...",
		},
	}}
	note := StopNoteFrom(logs)
	if note == nil || note.Plugin != "ConsoleUtilSSE" || note.Loader != "SKSE" {
		t.Fatalf("got %+v", note)
	}
}

func TestStopNoteIgnoresTheGameItself(t *testing.T) {
	logs := []LoaderLog{{
		Loader: "RED4ext",
		Path:   "/games/Cyb2077/red4ext/logs/red4ext-1.log",
		Lines: []string{
			"Loading plugin from 'X:\\Cyb2077\\bin\\x64\\Cyberpunk2077.exe'...",
			"Scripts blob prepared",
		},
	}}
	note := StopNoteFrom(logs)
	if note == nil || note.Plugin != "" {
		t.Fatalf("the game's own exe is not a plugin: %+v", note)
	}
}
