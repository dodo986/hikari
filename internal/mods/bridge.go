package mods

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hikari/internal/engine"
)

//go:embed bridge/modbridge.py
var bridgeScript []byte

//go:embed bridge/requirements.txt
var bridgeRequirements []byte

// Amethyst packaged by the distro keeps its Python tree in one of these.
var sourceCandidates = []string{
	"/usr/share/amethyst-mod-manager/src",
	"/usr/lib/amethyst-mod-manager/src",
	"/opt/amethyst-mod-manager/src",
}

const (
	probeTimeout = 90 * time.Second
	bridgeDir    = "bridge"
)

type Options struct {
	Root      string
	Source    string
	Python    string
	Filegraph string
	Log       *slog.Logger
	Runner    *engine.Runner
}

// Bridge runs Amethyst's own backend as a child process. One job at a time is
// the daemon's rule, not the bridge's.
type Bridge struct {
	log    *slog.Logger
	runner *engine.Runner
	root   string

	mu        sync.Mutex
	python    string
	source    string
	filegraph string
	blocked   string
	status    *Status
}

func New(opts Options) *Bridge {
	b := &Bridge{
		log:       opts.Log,
		runner:    opts.Runner,
		root:      opts.Root,
		python:    opts.Python,
		source:    opts.Source,
		filegraph: opts.Filegraph,
	}
	if b.log == nil {
		b.log = slog.Default()
	}
	if b.runner == nil {
		b.runner = engine.NewRunner(b.log)
	}
	b.prepare()
	return b
}

func (b *Bridge) scriptPath() string { return filepath.Join(b.root, bridgeDir, "modbridge.py") }

func (b *Bridge) requirementsPath() string {
	return filepath.Join(b.root, "requirements.txt")
}

func (b *Bridge) prepare() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.python == "" {
		if venv := filepath.Join(b.root, "venv", "bin", "python"); isFile(venv) {
			b.python = venv
		} else if path, err := exec.LookPath("python3"); err == nil {
			b.python = path
		}
	}
	if b.source == "" {
		b.source = os.Getenv("HIKARI_MODS_SOURCE")
	}
	if b.source == "" {
		for _, candidate := range sourceCandidates {
			if isFile(filepath.Join(candidate, "cli.py")) {
				b.source = candidate
				break
			}
		}
	}
	if b.filegraph == "" {
		for _, dir := range []string{b.root, b.source} {
			if dir == "" {
				continue
			}
			for _, name := range []string{"amethyst_filegraph.abi3.so", "amethyst_filegraph.so"} {
				if path := filepath.Join(dir, name); isFile(path) {
					b.filegraph = path
					break
				}
			}
			if b.filegraph != "" {
				break
			}
		}
	}

	switch {
	case b.python == "":
		b.blocked = "python3 was not found; install Python 3.11 or newer and run `hikari mods setup`"
	case b.source == "":
		b.blocked = "Amethyst was not found; set [mods] source_dir in settings.toml to its src directory"
	default:
		if err := b.materialize(); err != nil {
			b.blocked = fmt.Sprintf("cannot write the mods bridge into %s: %v", b.root, err)
		} else {
			b.blocked = ""
		}
	}
	b.status = nil
}

func (b *Bridge) materialize() error {
	if !isFile(filepath.Join(b.source, "cli.py")) {
		return fmt.Errorf("%s does not look like Amethyst's src directory", b.source)
	}
	dir := filepath.Join(b.root, bridgeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for path, content := range map[string][]byte{
		b.scriptPath():       bridgeScript,
		b.requirementsPath(): bridgeRequirements,
	} {
		if err := writeIfChanged(path, content); err != nil {
			return err
		}
	}
	return nil
}

func writeIfChanged(path string, content []byte) error {
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, content) {
		return nil
	}
	return os.WriteFile(path, content, 0o644)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

type config struct {
	python    string
	source    string
	filegraph string
	blocked   string
}

func (b *Bridge) snapshot() config {
	b.mu.Lock()
	defer b.mu.Unlock()
	return config{python: b.python, source: b.source, filegraph: b.filegraph, blocked: b.blocked}
}

func (b *Bridge) Status(ctx context.Context) Status {
	b.mu.Lock()
	cached, source := b.status, b.source
	b.mu.Unlock()
	if cached != nil {
		return *cached
	}
	cfg := b.snapshot()
	if cfg.blocked != "" {
		return Status{Reason: cfg.blocked, Source: source}
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "doctor", nil, Sink{})
	st := Status{Source: source}
	if err != nil {
		st.Reason = err.Error()
		return st
	}
	var out struct {
		Python    string `json:"python"`
		Source    string `json:"src"`
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		st.Reason = "the Amethyst backend sent an unreadable status"
		return st
	}
	st.Python, st.Available, st.Reason = out.Python, out.Available, out.Reason
	if out.Source != "" {
		st.Source = out.Source
	}
	if st.Available {
		b.mu.Lock()
		b.status = &st
		b.mu.Unlock()
	}
	return st
}

func (b *Bridge) Games(ctx context.Context) ([]Game, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "games", nil, Sink{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Games []Game `json:"games"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable game list"}
	}
	return out.Games, nil
}

func (b *Bridge) Deploy(ctx context.Context, game, profile string, sink Sink) error {
	_, err := b.run(ctx, "deploy", gameArgs(game, profile), sink)
	return err
}

func (b *Bridge) Restore(ctx context.Context, game string, sink Sink) error {
	_, err := b.run(ctx, "restore", gameArgs(game, ""), sink)
	return err
}

func (b *Bridge) Install(ctx context.Context, req InstallRequest, sink Sink) (string, error) {
	if req.Archive == "" && req.URL == "" {
		return "", &Error{Kind: KindUsage, Message: "install needs an archive path or an nxm:// url"}
	}
	args := gameArgs(req.Game, req.Profile)
	if req.Archive != "" {
		args = append(args, "--archive", req.Archive)
	}
	if req.URL != "" {
		args = append(args, "--url", req.URL)
	}
	if req.Name != "" {
		args = append(args, "--name", req.Name)
	}
	raw, err := b.run(ctx, "install", args, sink)
	if err != nil {
		return "", err
	}
	var out struct {
		Mod string `json:"mod"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable install result"}
	}
	return out.Mod, nil
}

func gameArgs(game, profile string) []string {
	args := []string{"--game", game}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	return args
}

// Setup gives the backend its own Python environment, so the system one stays untouched.
func (b *Bridge) Setup(ctx context.Context, sink Sink) error {
	venv := filepath.Join(b.root, "venv")
	python := filepath.Join(venv, "bin", "python")
	base := b.snapshot().python
	if base == "" || base == python {
		base, _ = exec.LookPath("python3")
	}
	if base == "" {
		return &Error{Kind: KindConfig, Message: "python3 was not found; install Python 3.11 or newer first"}
	}
	if err := b.materialize(); err != nil {
		return &Error{Kind: KindConfig, Message: fmt.Sprintf("cannot write the mods bridge into %s: %v", b.root, err)}
	}
	for _, argv := range [][]string{
		{base, "-m", "venv", venv},
		{python, "-m", "pip", "install", "--no-input", "--disable-pip-version-check", "--upgrade", "-r", b.requirementsPath()},
	} {
		sink.log(strings.Join(argv, " "))
		res := b.runner.Run(ctx, argv[0], argv[1:], engine.RunOptions{
			Env:      map[string]string{"PYTHONUNBUFFERED": "1", "PIP_DISABLE_PIP_VERSION_CHECK": "1"},
			Sanitize: func(line string) string { return line },
			OnLine: func(line string) {
				if strings.TrimSpace(line) != "" {
					sink.log(line)
				}
			},
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if res.Err != nil {
			return &Error{
				Kind:    KindFailure,
				Message: fmt.Sprintf("installing Amethyst's Python packages failed: %s", lastLine(res.Stderr)),
			}
		}
	}
	b.mu.Lock()
	b.python = python
	b.mu.Unlock()
	b.prepare()
	return nil
}

func (b *Bridge) run(ctx context.Context, command string, args []string, sink Sink) (json.RawMessage, error) {
	cfg := b.snapshot()
	if cfg.blocked != "" {
		return nil, &Error{Kind: KindUnavailable, Message: cfg.blocked}
	}
	env := map[string]string{
		"HIKARI_AMETHYST_SRC":     cfg.source,
		"PYTHONDONTWRITEBYTECODE": "1",
		"PYTHONUNBUFFERED":        "1",
	}
	if cfg.filegraph != "" {
		env["HIKARI_AMETHYST_EXT"] = cfg.filegraph
	}
	if os.Getenv("HIKARI_MODS_DOWNLOADS") == "" {
		env["HIKARI_MODS_DOWNLOADS"] = filepath.Join(b.root, "downloads")
	}

	var (
		mu      sync.Mutex
		result  json.RawMessage
		failure *Error
	)
	argv := append([]string{b.scriptPath(), command}, args...)
	res := b.runner.Run(ctx, cfg.python, argv, engine.RunOptions{
		Env: env,
		OnLine: func(line string) {
			event, ok := parseEvent(line)
			if !ok {
				sink.log(line)
				return
			}
			switch event.Type {
			case "log":
				sink.log(event.Message)
			case "progress":
				sink.report(Progress{Current: event.Current, Total: event.Total, Message: event.Message})
			case "result":
				mu.Lock()
				result = event.Data
				mu.Unlock()
			case "error":
				mu.Lock()
				failure = &Error{Kind: event.Kind, Message: event.Message}
				mu.Unlock()
			}
		},
	})
	mu.Lock()
	payload, reported := result, failure
	mu.Unlock()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if reported != nil {
		return nil, reported
	}
	if res.Err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend stopped: " + lastLine(res.Stderr)}
	}
	if len(payload) == 0 {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend produced no result: " + lastLine(res.Stderr)}
	}
	return payload, nil
}

type event struct {
	Type    string          `json:"t"`
	Message string          `json:"m"`
	Current int             `json:"cur"`
	Total   int             `json:"total"`
	Kind    string          `json:"kind"`
	Data    json.RawMessage `json:"data"`
}

func parseEvent(line string) (event, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return event{}, false
	}
	var ev event
	if err := json.Unmarshal([]byte(trimmed), &ev); err != nil || ev.Type == "" {
		return event{}, false
	}
	return ev, true
}

const maxLineLen = 400

func lastLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		if len(line) > maxLineLen {
			line = line[:maxLineLen] + "…"
		}
		return line
	}
	return "no output"
}
