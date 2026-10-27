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
	"strconv"
	"strings"
	"sync"
	"time"

	"hikari/internal/engine"
	"hikari/internal/mods/backend"
)

//go:embed bridge/modbridge.py
var bridgeScript []byte

//go:embed bridge/requirements.txt
var bridgeRequirements []byte

var filegraphNames = []string{"amethyst_filegraph.abi3.so", "amethyst_filegraph.so"}

const (
	probeTimeout = 90 * time.Second
	// nexusTimeout covers one API round trip, thumbnails excluded.
	nexusTimeout = 45 * time.Second
	// loginTimeout covers the user reading and approving a browser page.
	loginTimeout = 6 * time.Minute
	bridgeDir    = "bridge"
)

type Options struct {
	Root      string
	Home      string
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
	home   string

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
		home:      opts.Home,
		python:    opts.Python,
		filegraph: opts.Filegraph,
	}
	if b.log == nil {
		b.log = slog.Default()
	}
	if b.home == "" {
		b.home, _ = os.UserHomeDir()
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
	// A checkout named here is a development override: the shipped tree is
	// the embedded one, materialized below.
	if override := os.Getenv("HIKARI_MODS_SOURCE"); override != "" {
		b.source = asSourceDir(override)
	}
	switch {
	case b.python == "":
		b.blocked = "python3 was not found; install Python 3.12 or newer and run `hikari mods setup`"
	default:
		if err := b.materialize(); err != nil {
			b.blocked = fmt.Sprintf("cannot write the mods backend into %s: %v", b.root, err)
		} else {
			b.blocked = ""
			if b.source == "" {
				b.source = filepath.Join(backendRoot(b.root), backend.Src)
			}
			b.filegraph = firstFilegraph(b.root, b.filegraph)
			migrateState(b.root, b.home, b.log)
		}
	}
	b.status = nil
}

// ApplySettings re-reads the mods settings; the daemon calls it when settings.toml changes.
func (b *Bridge) ApplySettings(python, filegraph string) {
	b.mu.Lock()
	b.python, b.filegraph, b.status = python, filegraph, nil
	b.mu.Unlock()
	b.prepare()
}

// asSourceDir takes either Amethyst's src directory or its checkout root.
func asSourceDir(dir string) string {
	if dir == "" || isFile(filepath.Join(dir, "cli.py")) {
		return dir
	}
	if src := filepath.Join(dir, "src"); isFile(filepath.Join(src, "cli.py")) {
		return src
	}
	return dir
}

// filegraphDirs are the places a packaged or locally built module can sit.
func filegraphDirs(root, source string) []string {
	dirs := []string{root}
	if source != "" {
		dirs = append(dirs, filepath.Join(source, "..", "native", "amethyst_filegraph", "target", "release"))
	}
	return append(dirs, "/usr/lib/hikari", "/usr/share/hikari")
}

// firstFilegraph picks the first native module on hand, keeping a configured one.
func firstFilegraph(root, configured string) string {
	if configured != "" && isFile(configured) {
		return configured
	}
	for _, dir := range filegraphDirs(root, "") {
		for _, name := range filegraphNames {
			if path := filepath.Join(dir, name); isFile(path) {
				return path
			}
		}
	}
	return ""
}

// adoptFilegraph copies the native module next to the bridge when one is found
// elsewhere, so the daemon keeps working after the source tree moves away.
func (b *Bridge) adoptFilegraph() error {
	cfg := b.snapshot()
	if cfg.filegraph != "" && isFile(cfg.filegraph) {
		return nil
	}
	for _, dir := range append([]string{cfg.source}, filegraphDirs(b.root, cfg.source)...) {
		for _, name := range filegraphNames {
			src := filepath.Join(dir, name)
			if !isFile(src) {
				continue
			}
			dst := filepath.Join(b.root, name)
			if src != dst {
				content, err := os.ReadFile(src)
				if err != nil {
					return fmt.Errorf("cannot read the filegraph module from %s: %v", src, err)
				}
				if err := writeIfChanged(dst, content); err != nil {
					return fmt.Errorf("cannot install the filegraph module into %s: %v", b.root, err)
				}
			}
			b.mu.Lock()
			b.filegraph = dst
			b.mu.Unlock()
			return nil
		}
	}
	return fmt.Errorf("the native filegraph module is missing; run `scripts/build-backend.sh` " +
		"to build it from internal/mods/backend/native/amethyst_filegraph")
}

func (b *Bridge) materialize() error {
	dir := filepath.Join(b.root, bridgeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := materializeBackend(b.root); err != nil {
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

func (b *Bridge) Configure(ctx context.Context, req ConfigureRequest) (Configured, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	args := []string{"--game", req.Game, "--path", req.Path}
	if req.Staging != "" {
		args = append(args, "--staging", req.Staging)
	}
	if req.Prefix != "" {
		args = append(args, "--prefix", req.Prefix)
	}
	raw, err := b.run(probeCtx, "configure", args, Sink{})
	if err != nil {
		return Configured{}, err
	}
	var out Configured
	if err := json.Unmarshal(raw, &out); err != nil {
		return Configured{}, &Error{Kind: KindFailure, Message: "the modding backend sent an unreadable configuration"}
	}
	return out, nil
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
	if req.Archive == "" && req.URL == "" && (req.ModID <= 0 || req.FileID <= 0) {
		return "", &Error{Kind: KindUsage, Message: "install needs an archive path, an nxm:// url or a Nexus mod id and file id"}
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
	if req.ModID > 0 && req.FileID > 0 {
		args = append(args, "--mod-id", strconv.Itoa(req.ModID), "--file-id", strconv.Itoa(req.FileID))
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

// Nexus reads, stores or clears the API key in Amethyst's own keyring entry.
func (b *Bridge) Nexus(ctx context.Context, action, key string) (bool, error) {
	if action == "" {
		action = "status"
	}
	var env map[string]string
	if action == "set" {
		if strings.TrimSpace(key) == "" {
			return false, &Error{Kind: KindUsage, Message: "an API key is needed to store one"}
		}
		env = map[string]string{"HIKARI_NEXUS_KEY": strings.TrimSpace(key)}
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	raw, err := b.runWith(probeCtx, "nexus", []string{"--action", action}, Sink{}, env)
	if err != nil {
		return false, err
	}
	var out struct {
		Stored bool `json:"stored"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable result"}
	}
	return out.Stored, nil
}

// Account reports who the stored Nexus key belongs to and whether it may pull files.
func (b *Bridge) Account(ctx context.Context) (Account, error) {
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "account", nil, Sink{})
	if err != nil {
		return Account{}, err
	}
	var out Account
	if err := json.Unmarshal(raw, &out); err != nil {
		return Account{}, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable account"}
	}
	return out, nil
}

// Browse runs one page of Nexus search against the game's own domain.
// Login runs Amethyst's own browser login and keeps it company until it ends.
func (b *Bridge) Login(ctx context.Context, sink Sink) (Account, error) {
	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	raw, err := b.run(loginCtx, "login", nil, sink)
	if err != nil {
		return Account{}, err
	}
	var out Account
	if err := json.Unmarshal(raw, &out); err != nil {
		return Account{}, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable account"}
	}
	return out, nil
}

// Logout forgets the stored browser login, leaving any pasted key alone.
func (b *Bridge) Logout(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	_, err := b.run(probeCtx, "logout", nil, Sink{})
	return err
}

func (b *Bridge) Browse(ctx context.Context, game, query, sort string, page int) (Browse, error) {
	args := gameArgs(game, "")
	if query != "" {
		args = append(args, "--query", query)
	}
	if sort != "" {
		args = append(args, "--sort", sort)
	}
	if page > 0 {
		args = append(args, "--page", strconv.Itoa(page))
	}
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "search", args, Sink{})
	if err != nil {
		return Browse{}, err
	}
	var out Browse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Browse{}, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable search page"}
	}
	if out.Results == nil {
		out.Results = []Mod{}
	}
	return out, nil
}

// Files lists what can be downloaded for one mod.
func (b *Bridge) Files(ctx context.Context, game string, modID int) (Mod, []ModFile, error) {
	if modID <= 0 {
		return Mod{}, nil, &Error{Kind: KindUsage, Message: "mod id missing"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "files", append(gameArgs(game, ""), "--mod-id", strconv.Itoa(modID)), Sink{})
	if err != nil {
		return Mod{}, nil, err
	}
	var out struct {
		Mod   Mod       `json:"mod"`
		Files []ModFile `json:"files"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Mod{}, nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable file list"}
	}
	if out.Files == nil {
		out.Files = []ModFile{}
	}
	return out.Mod, out.Files, nil
}

// Toggle flips one modlist row; the caller deploys to apply it.
func (b *Bridge) Toggle(ctx context.Context, game, profile, name string) (bool, error) {
	if name == "" {
		return false, &Error{Kind: KindUsage, Message: "which mod?"}
	}
	args := append(gameArgs(game, profile), "--name", name)
	out, err := b.run(ctx, "toggle", args, Sink{})
	if err != nil {
		return false, err
	}
	var res struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return false, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable toggle answer"}
	}
	return res.Enabled, nil
}

// Remove deletes a mod's files, plugins and modlist row.
func (b *Bridge) Remove(ctx context.Context, game, profile, name string, sink Sink) error {
	if name == "" {
		return &Error{Kind: KindUsage, Message: "which mod?"}
	}
	_, err := b.run(ctx, "remove", append(gameArgs(game, profile), "--name", name), sink)
	return err
}

// Missing lists requirements the installed mods ask for and the profile lacks.
func (b *Bridge) Missing(ctx context.Context, game, profile string) ([]Missing, error) {
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	out, err := b.run(probeCtx, "missing", gameArgs(game, profile), Sink{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Missing []Missing `json:"missing"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable requirement list"}
	}
	if res.Missing == nil {
		res.Missing = []Missing{}
	}
	return res.Missing, nil
}

// Overrides are the Wine DLL overrides a game's mods need to load at all.
func (b *Bridge) Overrides(ctx context.Context, game string) (map[string]string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out, err := b.run(probeCtx, "overrides", gameArgs(game, ""), Sink{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Overrides map[string]string `json:"overrides"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent unreadable DLL overrides"}
	}
	return res.Overrides, nil
}

// Health asks the backend which loader files are missing or never ran.
func (b *Bridge) Health(ctx context.Context, game, profile string, network bool) ([]Finding, error) {
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	args := gameArgs(game, profile)
	if network {
		args = append(args, "--network")
	}
	out, err := b.run(probeCtx, "health", args, Sink{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable health report"}
	}
	return res.Findings, nil
}

// StopNote reads the logs a launch wrote, to explain a game that ended early.
func (b *Bridge) StopNote(ctx context.Context, root string, from, to time.Time) (*StopNote, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	args := []string{"--root", root,
		"--after", strconv.FormatInt(from.Unix(), 10),
		"--until", strconv.FormatInt(to.Add(launchSlack).Unix(), 10)}
	out, err := b.run(probeCtx, "lastlog", args, Sink{})
	if err != nil {
		return nil, err
	}
	var res struct {
		Logs []LoaderLog `json:"logs"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend sent unreadable loader logs"}
	}
	return StopNoteFrom(res.Logs), nil
}

// Downloads reports the archive of a browsed file that already finished
// downloading in the browser, so a free account can install it without
// copying a link around.
func (b *Bridge) Downloads(ctx context.Context, game string, modID, fileID int) (string, error) {
	if modID <= 0 {
		return "", &Error{Kind: KindUsage, Message: "mod id missing"}
	}
	args := append(gameArgs(game, ""), "--mod-id", strconv.Itoa(modID))
	if fileID > 0 {
		args = append(args, "--file-id", strconv.Itoa(fileID))
	}
	probeCtx, cancel := context.WithTimeout(ctx, nexusTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "downloads", args, Sink{})
	if err != nil {
		return "", err
	}
	var out struct {
		Found string `json:"found"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable download answer"}
	}
	return out.Found, nil
}

// Installed reads a profile's modlist, highest priority first.
func (b *Bridge) Installed(ctx context.Context, game, profile string) (Installed, error) {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	raw, err := b.run(probeCtx, "installed", gameArgs(game, profile), Sink{})
	if err != nil {
		return Installed{}, err
	}
	var out Installed
	if err := json.Unmarshal(raw, &out); err != nil {
		return Installed{}, &Error{Kind: KindFailure, Message: "the Amethyst backend sent an unreadable modlist"}
	}
	if out.Mods == nil {
		out.Mods = []InstalledMod{}
	}
	return out, nil
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
	b.prepare()
	if blocked := b.snapshot().blocked; blocked != "" {
		return &Error{Kind: KindConfig, Message: blocked}
	}
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
	if err := b.adoptFilegraph(); err != nil {
		return &Error{Kind: KindConfig, Message: err.Error()}
	}
	b.prepare()
	return nil
}

func (b *Bridge) run(ctx context.Context, command string, args []string, sink Sink) (json.RawMessage, error) {
	return b.runWith(ctx, command, args, sink, nil)
}

func (b *Bridge) runWith(ctx context.Context, command string, args []string, sink Sink, extra map[string]string) (json.RawMessage, error) {
	cfg := b.snapshot()
	if cfg.blocked != "" {
		return nil, &Error{Kind: KindUnavailable, Message: cfg.blocked}
	}
	env := map[string]string{
		"HIKARI_BACKEND_SRC":      cfg.source,
		"PYTHONDONTWRITEBYTECODE": "1",
		"PYTHONUNBUFFERED":        "1",
	}
	if cfg.filegraph != "" {
		env["HIKARI_BACKEND_EXT"] = cfg.filegraph
	}
	if os.Getenv("HIKARI_MODS_DOWNLOADS") == "" {
		env["HIKARI_MODS_DOWNLOADS"] = filepath.Join(b.root, "downloads")
	}
	for name, value := range stateEnv(b.root) {
		env[name] = value
	}
	for name, value := range extra {
		env[name] = value
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
			case "url":
				sink.authURL(event.URL)
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
		if strings.HasPrefix(res.Err.Error(), "cannot start ") {
			return nil, &Error{Kind: KindConfig, Message: fmt.Sprintf(
				"the modding backend could not start %s; set [mods] python in settings.toml to a Python 3.12 or newer",
				cfg.python)}
		}
		return nil, &Error{Kind: KindFailure, Message: "the modding backend stopped: " + lastLine(res.Stderr)}
	}
	if len(payload) == 0 {
		return nil, &Error{Kind: KindFailure, Message: "the Amethyst backend produced no result: " + lastLine(res.Stderr)}
	}
	return payload, nil
}

type event struct {
	Type    string          `json:"t"`
	Message string          `json:"m"`
	URL     string          `json:"u"`
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
