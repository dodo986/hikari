package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"hikari/internal/config"
)

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("process-group tests are Linux-only")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func testRunner() *Runner { return NewRunner(slog.New(slog.DiscardHandler)) }

func TestRunnerCapturesOutputAndEnv(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "fake-engine", `
echo "line one"
echo "line two" >&2
env | grep '^HIKARI_TEST='
`)
	r := testRunner()
	var lines []string
	res := r.Run(context.Background(), bin, nil, RunOptions{
		Env:    map[string]string{"HIKARI_TEST": "present"},
		OnLine: func(l string) { lines = append(lines, l) },
	})
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if !strings.Contains(res.Stdout, "line one") || !strings.Contains(res.Stdout, "HIKARI_TEST=present") {
		t.Fatalf("stdout wrong: %q", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "line two") {
		t.Fatalf("stderr wrong: %q", res.Stderr)
	}
	if len(lines) != 3 {
		t.Fatalf("OnLine saw %d lines, want 3: %q", len(lines), lines)
	}
}

func TestRunnerSanitizesOnLineAndLogs(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "leaky", `echo "auth --code SUPERSECRET"; echo "keep me"`)
	r := testRunner()
	var seen []string
	res := r.Run(context.Background(), bin, nil, RunOptions{
		Sanitize: func(l string) string { return strings.ReplaceAll(l, "SUPERSECRET", "<redacted>") },
		OnLine:   func(l string) { seen = append(seen, l) },
	})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	for _, l := range seen {
		if strings.Contains(l, "SUPERSECRET") {
			t.Fatalf("secret leaked to OnLine: %q", l)
		}
	}
	if !strings.Contains(res.Stdout, "SUPERSECRET") {
		t.Fatal("sanitizer must not mutate the captured result, only the streamed/logged copy")
	}
}

func TestRunnerNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "failer", `echo "boom" >&2; exit 3`)
	res := testRunner().Run(context.Background(), bin, nil, RunOptions{})
	if res.Err == nil {
		t.Fatal("non-zero exit should surface as Err")
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit code = %d, want 3", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "boom") {
		t.Fatalf("stderr lost: %q", res.Stderr)
	}
}

func TestRunnerMissingBinary(t *testing.T) {
	res := testRunner().Run(context.Background(), "/nonexistent/hikari-fake", nil, RunOptions{})
	if res.Err == nil {
		t.Fatal("missing binary must error")
	}
	if !strings.Contains(res.Err.Error(), "cannot start") {
		t.Fatalf("error should be actionable: %v", res.Err)
	}
}

func TestRunnerCancelKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	bin := writeScript(t, dir, "spawner", `
sleep 30 &
child=$!
trap '' TERM
while kill -0 $child 2>/dev/null; do sleep 0.1; done
`)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(300 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	res := testRunner().Run(ctx, bin, nil, RunOptions{})
	if !res.Canceled {
		t.Fatal("canceled context should mark the result canceled")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("group kill took too long; the TERM trap was not overridden")
	}
}

func TestEnvForIsolation(t *testing.T) {
	paths := config.ResolveFrom(t.TempDir(), t.TempDir())
	env := EnvFor(Legendary, paths)
	if env["LEGENDARY_CONFIG_PATH"] != paths.EngineEpic {
		t.Fatalf("legendary env = %v", env)
	}
	env = EnvFor(Gogdl, paths)
	if env["GOGDL_CONFIG_PATH"] != paths.EngineGOG {
		t.Fatalf("gogdl env = %v", env)
	}
	env = EnvFor(Nile, paths)
	if env["NILE_CONFIG_PATH"] != paths.EngineAmazon {
		t.Fatalf("nile env = %v", env)
	}
	if EnvFor(Comet, paths) != nil {
		t.Fatal("comet needs no config isolation")
	}
}

func TestSyncBufferStaysBoundedAfterCaptureLimit(t *testing.T) {
	var b syncBuffer
	b.WriteString(strings.Repeat("x", maxCapture))
	for range 100 {
		_ = b.WriteByte('\n')
	}
	if got := b.buf.Len(); got != maxCapture {
		t.Fatalf("capture length = %d, want %d", got, maxCapture)
	}
}
