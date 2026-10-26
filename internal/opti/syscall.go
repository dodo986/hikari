package opti

import (
	"context"
	"os/exec"
)

var (
	execLookPath = exec.LookPath
	execCommand  = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, name, args...)
	}
)
