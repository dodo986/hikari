package launch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"hikari/internal/config"
	"hikari/internal/engine"
	"hikari/internal/providers/common"
)

const (
	StateLaunching = "launching"
	StatePlaying   = "playing"
	StateStopped   = "stopped"
	StateCrashed   = "crashed"
)

type Manager struct {
	log    *slog.Logger
	runner *engine.Runner

	mu      sync.Mutex
	running map[string]*runningGame
	last    map[string]*RunLog
}

type RunLog struct {
	Store     string    `json:"store"`
	ID        string    `json:"id"`
	Running   bool      `json:"running"`
	State     string    `json:"state"`
	ExitCode  int       `json:"exit_code"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Lines     []string  `json:"lines"`
}

const maxRunLines = 200

type RunningGame struct {
	Store     string    `json:"store"`
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Pid       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

type runningGame struct {
	store   string
	id      string
	state   string
	pid     int
	started time.Time
	cancel  context.CancelFunc
	stopped chan struct{}

	sidecars []*exec.Cmd
}

func NewManager(log *slog.Logger, runner *engine.Runner) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{log: log, runner: runner, running: map[string]*runningGame{}, last: map[string]*RunLog{}}
}

func (m *Manager) Launch(ctx context.Context, store, id string, plan *Plan, onState func(state string, pid int)) error {
	key := store + "/" + id
	m.mu.Lock()
	if _, busy := m.running[key]; busy {
		m.mu.Unlock()
		return fmt.Errorf("%s is already running", key)
	}
	runCtx, cancel := context.WithCancel(ctx)
	rg := &runningGame{
		store: store, id: id, state: StateLaunching,
		started: time.Now(), cancel: cancel, stopped: make(chan struct{}),
	}
	rec := &RunLog{Store: store, ID: id, Running: true, State: StateLaunching, StartedAt: time.Now()}
	m.last[key] = rec
	m.running[key] = rg
	m.mu.Unlock()

	if onState != nil {
		onState(StateLaunching, 0)
	}
	go func() {
		defer close(rg.stopped)

		if plan.Prepare != nil {
			if err := plan.Prepare(runCtx); err != nil {
				m.log.Error("launch preparation failed", "game", key, "err", err)
				m.finish(key, StateCrashed, 0, err)
				if onState != nil {
					onState(StateCrashed, 0)
				}
				return
			}
		}
		for _, sc := range plan.Sidecars {
			if len(sc.Argv) == 0 {
				continue
			}
			cmd := exec.Command(sc.Argv[0], sc.Argv[1:]...)
			cmd.Env = os.Environ()
			for k, v := range sc.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			if err := cmd.Start(); err != nil {
				m.log.Warn("sidecar failed to start", "name", sc.Name, "err", err)
				continue
			}
			m.mu.Lock()
			rg.sidecars = append(rg.sidecars, cmd)
			m.mu.Unlock()
		}
		res := m.runner.Run(runCtx, plan.Argv[0], plan.Argv[1:], engine.RunOptions{
			Env: plan.Env,
			Dir: plan.Cwd,
			OnLine: func(line string) {
				m.mu.Lock()
				if rec := m.last[key]; rec != nil {
					rec.Lines = append(rec.Lines, line)
					if len(rec.Lines) > maxRunLines {
						rec.Lines = rec.Lines[len(rec.Lines)-maxRunLines:]
					}
				}
				m.mu.Unlock()
			},
			OnStart: func(pid int) {
				m.mu.Lock()
				rg.pid = pid
				rg.state = StatePlaying
				m.mu.Unlock()
				if onState != nil {
					onState(StatePlaying, pid)
				}
			},
		})
		if !res.Canceled && plan.Prefix != "" {
			m.adoptPrefix(runCtx, plan.Prefix, func(pid int) {
				m.log.Debug("game outlived the engine", "game", key, "pid", pid)
				m.mu.Lock()
				rg.pid = pid
				rg.state = StatePlaying
				m.mu.Unlock()
				if onState != nil {
					onState(StatePlaying, pid)
				}
			})
		}
		for _, sc := range rg.sidecars {
			if sc.Process != nil {
				_ = sc.Process.Kill()
				_ = sc.Wait()
			}
		}
		err := res.Err
		if res.Canceled {
			err = nil
		} else if msg := common.FirstLines(res.Stderr, 1); msg != "" {
			err = errors.New(msg)
		}
		state := StateStopped
		if err != nil {
			state = StateCrashed
			m.log.Error("game exited with error", "game", key, "err", err)
		}
		m.finish(key, state, res.ExitCode, err)
		if onState != nil {
			onState(state, 0)
		}
	}()
	return nil
}

func (m *Manager) finish(key, state string, code int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.running, key)
	rec := m.last[key]
	if rec == nil {
		return
	}
	rec.Running = false
	rec.State = state
	rec.ExitCode = code
	rec.EndedAt = time.Now()
	if err != nil {
		rec.Error = err.Error()
	}
}

func (m *Manager) LastRun(store, id string) (RunLog, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.last[store+"/"+id]
	if !ok {
		return RunLog{}, false
	}
	cp := *rec
	cp.Lines = append([]string(nil), rec.Lines...)
	return cp, true
}

const prefixPollInterval = 2 * time.Second

func (m *Manager) adoptPrefix(ctx context.Context, prefix string, onPlay func(pid int)) {
	pid, ok := prefixPID(prefix)
	if !ok {
		select {
		case <-ctx.Done():
			return
		case <-time.After(prefixPollInterval):
		}
		if pid, ok = prefixPID(prefix); !ok {
			return
		}
	}
	onPlay(pid)
	t := time.NewTicker(prefixPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, alive := prefixPID(prefix); !alive {
				m.log.Debug("game left the prefix", "prefix", prefix)
				return
			}
		}
	}
}

func prefixPID(prefix string) (int, bool) {
	pids := PrefixProcesses(prefix)
	if len(pids) == 0 {
		return 0, false
	}
	return pids[0], true
}

func (m *Manager) Stop(key string) error {
	m.mu.Lock()
	rg := m.running[key]
	m.mu.Unlock()
	if rg == nil {
		return fmt.Errorf("%s is not running", key)
	}
	rg.cancel()
	select {
	case <-rg.stopped:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("%s did not stop within 5s; it may need SIGKILL", key)
	}
}

func (m *Manager) LaunchFake(ctx context.Context, store, id string, life time.Duration, onState func(state string, pid int)) error {
	key := store + "/" + id
	m.mu.Lock()
	if _, busy := m.running[key]; busy {
		m.mu.Unlock()
		return fmt.Errorf("%s is already running", key)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	rg := &runningGame{
		store: store, id: id, state: StatePlaying, pid: 4200 + len(key)%1000,
		started: time.Now(), cancel: cancel, stopped: make(chan struct{}),
	}
	rec := &RunLog{Store: store, ID: id, Running: true, State: StatePlaying, StartedAt: time.Now()}
	m.last[key] = rec
	m.running[key] = rg
	m.mu.Unlock()

	if onState != nil {
		onState(StateLaunching, 0)
		onState(StatePlaying, rg.pid)
	}
	go func() {
		defer close(rg.stopped)
		select {
		case <-runCtx.Done():
		case <-ctx.Done():
			cancel()
		case <-time.After(life):
			cancel()
		}
		m.finish(key, StateStopped, 0, nil)
		if onState != nil {
			onState(StateStopped, 0)
		}
	}()
	return nil
}

func (m *Manager) Running() []RunningGame {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RunningGame, 0, len(m.running))
	for _, rg := range m.running {
		out = append(out, RunningGame{
			Store: rg.store, ID: rg.id, State: rg.state,
			Pid: rg.pid, StartedAt: rg.started,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Store != out[j].Store {
			return out[i].Store < out[j].Store
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (m *Manager) IsRunning(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[key]
	return ok
}

func RunInPrefix(ctx context.Context, runner *engine.Runner, settings config.GameSettings, eng *engine.Manager, argv []string, cwd string) error {
	wt, err := selectWine(settings, eng)
	if err != nil {
		return err
	}
	env := map[string]string{"WINEPREFIX": settings.WinePrefix}
	var full []string
	switch wt {
	case WinePlain:
		bin := settings.WineVersion.Bin
		if bin == "" {
			var err error
			bin, err = eng.Require(engine.Wine)
			if err != nil {
				return err
			}
		}
		full = append([]string{bin}, argv...)
	case WineProtonUmu:
		umu, err := eng.Require(engine.Umu)
		if err != nil {
			return err
		}
		full = append([]string{umu}, argv...)
	case WineProtonDirect:
		full = append([]string{settings.WineVersion.Bin, "run"}, argv...)
	default:
		return fmt.Errorf("cannot run in prefix for wine type %q", wt)
	}
	res := runner.Run(ctx, full[0], full[1:], engine.RunOptions{Env: env, Dir: cwd})
	if res.Err != nil {
		return fmt.Errorf("command in prefix failed: %s", res.Stderr)
	}
	return nil
}
