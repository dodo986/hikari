package engine

import (
	"context"
	"strings"
	"testing"
)

// A line longer than the capture limit used to stop the reader: the pipe's read
// end was closed, the child died of SIGPIPE, and everything it printed
// afterwards was lost. The long line may be truncated, but the run must finish
// and the following lines must still be captured.
func TestRunnerSurvivesAVeryLongLine(t *testing.T) {
	dir := t.TempDir()
	// `echo` ends the two megabyte line; AFTER-MARKER is then a line of its own.
	bin := writeScript(t, dir, "noisy", "head -c 2000000 /dev/zero | tr '\\0' x\necho\necho AFTER-MARKER\n")
	res := testRunner().Run(context.Background(), bin, nil, RunOptions{})
	if res.Canceled {
		t.Fatal("the run was canceled")
	}
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("the child did not finish cleanly: exit=%d err=%v", res.ExitCode, res.Err)
	}
	if !strings.Contains(res.Stdout, "AFTER-MARKER") {
		t.Fatal("output printed after the long line was lost")
	}
	if !strings.Contains(res.Stdout, "[line truncated]") {
		t.Fatal("the over-long line was not marked as truncated")
	}
	if len(res.Stdout) > maxLineBytes+1024 {
		t.Fatalf("captured %d bytes, want the line capped near %d", len(res.Stdout), maxLineBytes)
	}
}

// The same on stderr: an engine that logs a huge blob must not lose the rest of
// its output either.
func TestRunnerDrainsALongStderrLine(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "noisy-err", "head -c 1500000 /dev/zero | tr '\\0' y >&2\necho >&2\necho AFTER-ERR >&2\necho done\n")
	res := testRunner().Run(context.Background(), bin, nil, RunOptions{})
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("the child did not finish cleanly: exit=%d err=%v", res.ExitCode, res.Err)
	}
	if !strings.Contains(res.Stderr, "AFTER-ERR") {
		t.Fatal("stderr printed after the long line was lost")
	}
	if !strings.Contains(res.Stdout, "done") {
		t.Fatal("stdout was lost")
	}
}
