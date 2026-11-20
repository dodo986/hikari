package config

import "testing"

func TestStateEntryReturnsASnapshot(t *testing.T) {
	st, err := LoadState(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEntry("epic", "baka", func(e *GameEntry) {
		e.PlayCount = 1
		e.Favorite = true
		e.Settings.WinePrefix = "/pfx/before"
	}); err != nil {
		t.Fatal(err)
	}

	snapshot := st.Entry("epic", "baka")
	if snapshot == nil {
		t.Fatal("entry missing")
	}

	if err := st.UpdateEntry("epic", "baka", func(e *GameEntry) {
		e.PlayCount = 5
		e.Favorite = false
		e.Settings.WinePrefix = "/pfx/after"
	}); err != nil {
		t.Fatal(err)
	}

	if snapshot.PlayCount != 1 || !snapshot.Favorite || snapshot.Settings.WinePrefix != "/pfx/before" {
		t.Fatalf("snapshot changed with the stored entry: %+v", snapshot)
	}
	fresh := st.Entry("epic", "baka")
	if fresh == snapshot {
		t.Fatal("Entry handed back the stored pointer")
	}
	if fresh.PlayCount != 5 || fresh.Favorite || fresh.Settings.WinePrefix != "/pfx/after" {
		t.Fatalf("stored entry not updated: %+v", fresh)
	}
}

func TestStateEntrySnapshotCopiesSettings(t *testing.T) {
	st, err := LoadState(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEntry("epic", "baka", func(e *GameEntry) {
		e.Settings.Environment = []EnvVar{{Key: "DXVK_HUD", Value: "fps"}}
		e.Settings.Wrappers = []Wrapper{{Exe: "/usr/bin/gamemoderun"}}
		e.Settings.Gamescope = &GamescopeSettings{GameWidth: "1920", GameHeight: "1080"}
	}); err != nil {
		t.Fatal(err)
	}

	snapshot := st.Entry("epic", "baka")
	if snapshot == nil {
		t.Fatal("entry missing")
	}
	snapshot.Settings.Environment[0].Value = "mutated"
	snapshot.Settings.Wrappers = append(snapshot.Settings.Wrappers, Wrapper{Exe: "/usr/bin/mangohud"})
	snapshot.Settings.Gamescope.GameWidth = "9999"

	fresh := st.Entry("epic", "baka")
	if got := fresh.Settings.Environment; len(got) != 1 || got[0].Value != "fps" {
		t.Fatalf("snapshot shares the Environment slice with the store: %+v", got)
	}
	if got := fresh.Settings.Wrappers; len(got) != 1 || got[0].Exe != "/usr/bin/gamemoderun" {
		t.Fatalf("snapshot shares the Wrappers slice with the store: %+v", got)
	}
	if fresh.Settings.Gamescope == nil || fresh.Settings.Gamescope.GameWidth != "1920" {
		t.Fatalf("snapshot shares the Gamescope pointer with the store: %+v", fresh.Settings.Gamescope)
	}
}
