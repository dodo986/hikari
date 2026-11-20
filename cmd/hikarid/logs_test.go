package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"hikari/internal/launch"
	"hikari/internal/mods"
)

func TestGameLogCarriesTheLoaderSentence(t *testing.T) {
	rec := launch.RunLog{
		State:     launch.StateCrashed,
		StartedAt: time.Now().Add(-time.Minute),
		EndedAt:   time.Now(),
		Lines:     []string{"Proton: Executable is a unix path"},
	}
	note := &mods.StopNote{
		Loader:  "RED4ext",
		Plugin:  "Codeware",
		Log:     "/games/Cyb2077/red4ext/logs/red4ext-1.log",
		Message: "The game stopped while RED4ext was loading Codeware",
		Fix:     "Disable Codeware in Mods and launch again",
	}
	raw, err := json.Marshal(logResponse{RunLog: rec, ModStop: note})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["state"] != launch.StateCrashed || got["lines"] == nil {
		t.Fatalf("the record lost its own fields: %s", raw)
	}
	stop, ok := got["mod_stop"].(map[string]any)
	if !ok || stop["plugin"] != "Codeware" || stop["message"] != note.Message {
		t.Fatalf("mod_stop missing or wrong: %s", raw)
	}
	if !strings.Contains(strings.ToLower(stop["fix"].(string)), "disable codeware") {
		t.Fatalf("fix line lost: %s", raw)
	}
}

func TestGameLogWithoutANoteStaysUnchanged(t *testing.T) {
	raw, err := json.Marshal(logResponse{RunLog: launch.RunLog{State: launch.StateStopped}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "mod_stop") {
		t.Fatalf("a healthy launch should not mention mods: %s", raw)
	}
}

// The stop note explains a launch that never got going. A long session says
// nothing about the loaders it got past, and neither a crashed one nor a clean
// one should be told "the game stopped while X was loading Y".
func TestStopNoteOnlyForShortRuns(t *testing.T) {
	now := time.Now()
	long := launch.RunLog{State: launch.StateCrashed, StartedAt: now.Add(-time.Hour), EndedAt: now}
	if wantsStopNote(long, now) {
		t.Fatal("an hour-long crashed session must not get the loader note")
	}
	long.State = launch.StateStopped
	if wantsStopNote(long, now) {
		t.Fatal("an hour-long clean exit must not get the loader note")
	}
	short := launch.RunLog{State: launch.StateCrashed, StartedAt: now.Add(-2 * time.Second), EndedAt: now}
	if !wantsStopNote(short, now) {
		t.Fatal("a crash two seconds in is exactly the case the note is for")
	}
	short.Running = true
	if wantsStopNote(short, now) {
		t.Fatal("a running game has no note yet")
	}
	short.Running = false
	short.Canceled = true
	if wantsStopNote(short, now) {
		t.Fatal("a game the player stopped is not a mystery")
	}
}
