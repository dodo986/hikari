package launch

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"hikari/internal/engine"
)

func startWithPrefix(t *testing.T, key, prefix string) int {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	cmd.Env = append(os.Environ(), key+"="+prefix)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd.Process.Pid
}

func containsPID(pids []int, pid int) bool {
	for _, p := range pids {
		if p == pid {
			return true
		}
	}
	return false
}

func TestPrefixProcessesMatchesBothSpellings(t *testing.T) {
	prefix := t.TempDir()
	plain := startWithPrefix(t, "WINEPREFIX", prefix)
	pfx := startWithPrefix(t, "WINEPREFIX", filepath.Join(prefix, "pfx")+"/")
	umu := startWithPrefix(t, "STEAM_COMPAT_DATA_PATH", prefix)
	other := startWithPrefix(t, "WINEPREFIX", filepath.Join(prefix, "other"))

	pids := PrefixProcesses(prefix)
	for name, pid := range map[string]int{"plain": plain, "pfx": pfx, "umu": umu} {
		if !containsPID(pids, pid) {
			t.Fatalf("%s process %d not found in %v", name, pid, pids)
		}
	}
	if containsPID(pids, other) {
		t.Fatalf("a process from another prefix matched: %v", pids)
	}
}

func TestManagerAdoptsSurvivingGameProcess(t *testing.T) {
	prefix := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "game.pid")
	script := "sleep 30 & echo $! > " + pidFile + "\n"
	bin := filepath.Join(t.TempDir(), "fake-engine")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(slog.New(slog.DiscardHandler), engine.NewRunner(slog.New(slog.DiscardHandler)))
	states := make(chan string, 8)
	plan := &Plan{Argv: []string{bin}, Env: map[string]string{"WINEPREFIX": prefix}, Prefix: prefix}
	if err := m.Launch(context.Background(), "epic", "g", plan, func(state string, pid int) {
		states <- state
	}); err != nil {
		t.Fatal(err)
	}

	gamePID := waitForPIDFile(t, pidFile)
	t.Cleanup(func() {
		if p, err := os.FindProcess(gamePID); err == nil {
			_ = p.Kill()
		}
	})

	playing := 0
	deadline := time.After(5 * time.Second)
	for playing < 2 {
		select {
		case got := <-states:
			if got == StatePlaying {
				playing++
			}
			if got == StateStopped {
				t.Fatal("session stopped while the game was still in the prefix")
			}
		case <-deadline:
			t.Fatal("the surviving game was never reported as playing")
		}
	}
	if !m.IsRunning("epic/g") {
		t.Fatal("the session must stay registered while the game runs")
	}

	if err := syscall.Kill(gamePID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline = time.After(6 * time.Second)
	for {
		select {
		case got := <-states:
			if got == StateStopped {
				if m.IsRunning("epic/g") {
					t.Fatal("stopped session still registered")
				}
				return
			}
		case <-deadline:
			t.Fatal("session did not end after the game left the prefix")
		}
	}
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no pid file at %s", path)
	return 0
}
