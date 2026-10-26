package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Canceled bool
	Err      error
}

type RunOptions struct {
	Env      map[string]string
	Dir      string
	OnLine   func(line string)
	Sanitize func(line string) string
	OnStart  func(pid int)
}

type Runner struct {
	Log *slog.Logger
}

func NewRunner(log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{Log: log}
}

func (r *Runner) Run(ctx context.Context, bin string, args []string, opts RunOptions) Result {
	sanitize := opts.Sanitize
	if sanitize == nil {
		sanitize = func(s string) string { return s }
	}
	display := sanitize(strings.Join(append([]string{bin}, args...), " "))
	r.Log.Debug("engine: running command", "cmd", display)

	cmd := exec.Command(bin, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = append(osEnviron(), envSlice(opts.Env)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Own pipes: a detached child (wineserver) can hold stdout open and never EOF.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return Result{Err: err}
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		stdoutR.Close()
		stdoutW.Close()
		return Result{Err: err}
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	if err := cmd.Start(); err != nil {
		stdoutR.Close()
		stdoutW.Close()
		stderrR.Close()
		stderrW.Close()
		return Result{Err: fmt.Errorf("cannot start %s: %w", bin, err)}
	}
	stdoutW.Close()
	stderrW.Close()
	if opts.OnStart != nil && cmd.Process != nil {
		opts.OnStart(cmd.Process.Pid)
	}

	// Tear the group down only on cancel: the game shares it after handoff.
	var finished atomic.Bool
	runDone := make(chan struct{})
	defer close(runDone)
	go func() {
		select {
		case <-runDone:
			return
		case <-ctx.Done():
			if finished.Load() || cmd.Process == nil {
				return
			}
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			select {
			case <-runDone:
			case <-time.After(3 * time.Second):
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
	}()

	var outBuf, errBuf syncBuffer
	var wg sync.WaitGroup

	var forward atomic.Bool
	forward.Store(true)

	var cbMu sync.Mutex
	onLine := func(line string) {
		if opts.OnLine == nil {
			return
		}
		cbMu.Lock()
		defer cbMu.Unlock()
		if forward.Load() {
			opts.OnLine(line)
		}
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer stdoutR.Close()
		stream(stdoutR, &outBuf, onLine, sanitize)
	}()
	go func() {
		defer wg.Done()
		defer stderrR.Close()
		stream(stderrR, &errBuf, onLine, sanitize)
	}()

	waitErr := cmd.Wait()
	finished.Store(true)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(drainGrace):
		forward.Store(false)
		cbMu.Lock()
		cbMu.Unlock()
	}

	res := Result{Stdout: outBuf.String(), Stderr: errBuf.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		res.Canceled = true
		return res
	}
	if waitErr != nil {
		res.Err = waitErr
	}
	return res
}

const drainGrace = 300 * time.Millisecond

func stream(pipe io.ReadCloser, buf *syncBuffer, onLine func(string), sanitize func(string) string) {
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		buf.WriteString(line)
		buf.WriteByte('\n')
		if onLine != nil {
			onLine(sanitize(line))
		}
	}
}

const maxCapture = 4 << 20

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) WriteString(s string) {
	b.mu.Lock()
	if b.buf.Len() < maxCapture {
		b.buf.WriteString(s)
	}
	b.mu.Unlock()
}

func (b *syncBuffer) WriteByte(c byte) error {
	b.mu.Lock()
	if b.buf.Len() < maxCapture {
		b.buf.WriteByte(c)
	}
	b.mu.Unlock()
	return nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
