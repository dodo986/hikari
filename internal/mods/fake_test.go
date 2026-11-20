package mods

import (
	"context"
	"sync"
	"testing"
)

// TestFakeFindReturnsACopy pins that a read cannot mutate the stored game.
func TestFakeFindReturnsACopy(t *testing.T) {
	fake := NewFake()
	game := fake.find("skyrim_se")
	if game == nil {
		t.Fatal("skyrim_se missing from the demo catalogue")
	}
	game.Configured = false
	game.Path = "/mutated"
	if again := fake.find("skyrim_se"); !again.Configured || again.Path != "/games/Skyrim Special Edition" {
		t.Fatalf("mutating the copy changed the stored game: %+v", again)
	}
}

// TestFakeConfigureAndReadsDoNotRace runs the writer against the readers so
// -race catches an unlocked read of the games slice.
func TestFakeConfigureAndReadsDoNotRace(t *testing.T) {
	fake := NewFake()
	const rounds = 300
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			if _, err := fake.Configure(context.Background(), ConfigureRequest{Game: "skyrim_se", Path: "/games/Skyrim Special Edition"}); err != nil {
				t.Errorf("Configure returned %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			if _, _, err := fake.Files(context.Background(), "skyrim_se", 12604); err != nil {
				t.Errorf("Files returned %v", err)
				return
			}
			if _, err := fake.Installed(context.Background(), "skyrim_se", "default"); err != nil {
				t.Errorf("Installed returned %v", err)
				return
			}
		}
	}()
	wg.Wait()
}
