package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"hikari/internal/atomicfile"
)

// Names of the secrets the UI may set. Anything else is rejected, so a caller
// cannot use this as a generic key-value store in the keyring.
const (
	SteamGridDB = "steamgriddb"
)

var known = map[string]string{
	SteamGridDB: "HIKARI_SGDB_KEY",
}

// Status tells the UI whether a secret exists and whether it is protected by
// the desktop keyring. The value itself never leaves the daemon.
type Status struct {
	Name      string `json:"name"`
	Set       bool   `json:"set"`
	Encrypted bool   `json:"encrypted"`
	Backend   string `json:"backend"`
}

// Store keeps API keys in the Secret Service, and only falls back to a 0600
// file when no keyring is running.
type Store struct {
	path string
}

func New(fallbackPath string) *Store { return &Store{path: fallbackPath} }

func Known(name string) bool {
	_, ok := known[name]
	return ok
}

// Get resolves the key: environment override first, then keyring, then the
// fallback file. The environment wins so a session can run without persisting.
func (s *Store) Get(name string) (string, bool) {
	if !Known(name) {
		return "", false
	}
	if v := strings.TrimSpace(os.Getenv(known[name])); v != "" {
		return v, true
	}
	if v, err := keyring.Get(service(), name); err == nil && v != "" {
		return v, true
	}
	if v, ok := s.readFile()[name]; ok && v != "" {
		return v, true
	}
	return "", false
}

func (s *Store) Set(name, value string) error {
	if !Known(name) {
		return fmt.Errorf("hikari: %q is not a secret this daemon stores", name)
	}
	if value == "" {
		return s.Delete(name)
	}
	if err := keyring.Set(service(), name, value); err == nil {
		s.deleteFileKey(name)
		return nil
	}
	if err := s.writeFileKey(name, value); err != nil {
		return fmt.Errorf("hikari: no keyring and cannot write %s: %w", s.path, err)
	}
	return nil
}

func (s *Store) Delete(name string) error {
	if !Known(name) {
		return fmt.Errorf("hikari: %q is not a secret this daemon stores", name)
	}
	errKeyring := keyring.Delete(service(), name)
	if errors.Is(errKeyring, keyring.ErrNotFound) {
		errKeyring = nil
	}
	errFile := s.deleteFileKey(name)
	if errKeyring != nil {
		return errKeyring
	}
	return errFile
}

func (s *Store) Status(name string) Status {
	st := Status{Name: name}
	if !Known(name) {
		return st
	}
	if v := strings.TrimSpace(os.Getenv(known[name])); v != "" {
		return Status{Name: name, Set: true, Encrypted: false, Backend: "environment"}
	}
	if v, err := keyring.Get(service(), name); err == nil && v != "" {
		return Status{Name: name, Set: true, Encrypted: true, Backend: "keyring"}
	}
	if v, ok := s.readFile()[name]; ok && v != "" {
		return Status{Name: name, Set: true, Encrypted: false, Backend: "file"}
	}
	if keyringAvailable() {
		return Status{Name: name, Backend: "keyring"}
	}
	return Status{Name: name, Backend: "file"}
}

// Encrypted reports whether new writes would be protected, so the UI can warn
// when a key would land in a plain file.
func (s *Store) Encrypted() bool { return keyringAvailable() }

// Migrate moves a plaintext settings value into the keyring and reports the
// settings value that should replace it.
func (s *Store) Migrate(name, plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", nil
	}
	if _, ok := s.Get(name); ok {
		return "", nil
	}
	if err := s.Set(name, plaintext); err != nil {
		return plaintext, err
	}
	return "", nil
}

func keyringAvailable() bool {
	if err := keyring.Set(service(), "probe", "1"); err != nil {
		return false
	}
	_ = keyring.Delete(service(), "probe")
	return true
}

func service() string { return "hikari" }

func (s *Store) readFile() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]string{}
	}
	return out
}

func (s *Store) writeFileKey(name, value string) error {
	m := s.readFile()
	m[name] = value
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteMode(s.path, append(b, '\n'), 0o600)
}

func (s *Store) deleteFileKey(name string) error {
	m := s.readFile()
	if _, ok := m[name]; !ok {
		return nil
	}
	delete(m, name)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteMode(s.path, append(b, '\n'), 0o600)
}
