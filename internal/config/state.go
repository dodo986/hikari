package config

import (
	"hikari/internal/atomicfile"

	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type GameEntry struct {
	Settings   GameSettings `json:"settings"`
	LastPlayed time.Time    `json:"last_played,omitempty"`
	PlayCount  int          `json:"play_count,omitempty"`

	SettingsComplete bool `json:"settings_complete,omitempty"`
	Imported         bool `json:"imported,omitempty"`
	Favorite         bool `json:"favorite,omitempty"`
}

type State struct {
	Version int                   `json:"version"`
	Games   map[string]*GameEntry `json:"games"`
}

type StateStore struct {
	mu    sync.Mutex
	path  string
	state State
	dirty bool
}

func LoadState(paths Paths) (*StateStore, error) {
	st := &StateStore{path: paths.State, state: State{Version: 1, Games: map[string]*GameEntry{}}}
	b, err := os.ReadFile(paths.State)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return nil, fmt.Errorf("hikari: cannot read %s: %w", paths.State, err)
	}
	if err := json.Unmarshal(b, &st.state); err != nil {
		return nil, fmt.Errorf("hikari: %s is corrupt: %w", paths.State, err)
	}
	if st.state.Games == nil {
		st.state.Games = map[string]*GameEntry{}
	}
	return st, nil
}

func GameKey(store, id string) string { return store + "/" + id }

func (s *StateStore) Entry(store, id string) *GameEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Games[GameKey(store, id)]
}

func (s *StateStore) PutEntry(store, id string, e *GameEntry) error {
	s.mu.Lock()
	s.state.Games[GameKey(store, id)] = e
	err := s.saveLocked()
	s.mu.Unlock()
	return err
}

func (s *StateStore) UpdateEntry(store, id string, fn func(e *GameEntry)) error {
	s.mu.Lock()
	key := GameKey(store, id)
	e := s.state.Games[key]
	if e == nil {
		e = &GameEntry{}
		s.state.Games[key] = e
	}
	fn(e)
	err := s.saveLocked()
	s.mu.Unlock()
	return err
}

func (s *StateStore) saveLocked() error {
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(s.path, append(b, '\n'))
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}
