package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunnerLeavesGroupAliveOnNormalExit(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	bin := writeScript(t, dir, "hands-off", "\nsleep 30 &\necho $! > "+pidFile+"\nexit 0\n")
	res := testRunner().Run(context.Background(), bin, nil, RunOptions{})
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("result: %+v", res)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("no child pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("bad child pid %q", data)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("child %d was killed by the run ending: %v", pid, err)
	}
}

func TestRunnerCancelKillsGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	bin := writeScript(t, dir, "slow", "\nsleep 30 &\necho $! > "+pidFile+"\nwait\n")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			if _, err := os.Stat(pidFile); err == nil {
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	res := testRunner().Run(ctx, bin, nil, RunOptions{})
	if !res.Canceled {
		t.Fatalf("result should be canceled: %+v", res)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("no child pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("bad child pid %q", data)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("child %d survived cancellation", pid)
}
