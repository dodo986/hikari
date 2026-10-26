package engine

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestRunnerEndsWhenEngineExitsDespiteDetachedChild(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is needed to detach a grandchild")
	}
	dir := t.TempDir()
	bin := writeScript(t, dir, "daemonizing", `
setsid sleep 30 &
echo started
exit 0
`)
	r := testRunner()
	var lines []string
	start := time.Now()
	res := r.Run(context.Background(), bin, nil, RunOptions{
		OnLine: func(l string) { lines = append(lines, l) },
	})
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("run waited for the detached child: %v", elapsed)
	}
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("result: %+v", res)
	}
	if res.Stdout == "" || len(lines) != 1 || lines[0] != "started" {
		t.Fatalf("output before exit was lost: stdout=%q lines=%q", res.Stdout, lines)
	}
}
