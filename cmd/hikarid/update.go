package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"hikari/internal/api"
	"hikari/internal/archive"
	"hikari/internal/config"
	"hikari/internal/update"
)

var updateBusy atomic.Bool

func (d *daemon) updateChecker() *update.Checker {
	c := update.New(os.Getenv("HIKARI_UPDATE_REPO"))
	if base := os.Getenv("HIKARI_UPDATE_API"); base != "" {
		c.Base = base
	}
	if base := os.Getenv("HIKARI_UPDATE_CODELOAD"); base != "" {
		c.Codeload = base
	}
	if branch := os.Getenv("HIKARI_UPDATE_BRANCH"); branch != "" {
		c.Branch = branch
	}
	return c
}

func appImagePath() string {
	for _, key := range []string{"HIKARI_APPIMAGE", "APPIMAGE"} {
		if p := os.Getenv(key); p != "" {
			return p
		}
	}
	return ""
}

func (d *daemon) registerUpdate(srv *api.Server) {
	srv.Register("app.update.check", func(c context.Context, params json.RawMessage) (any, error) {
		info, err := d.updateChecker().Check(c, d.version, d.commit)
		if err != nil {
			return nil, api.Errorf(api.CodeNotAvailable, "%s", err)
		}
		return info, nil
	})
	srv.Register("app.update.apply", func(c context.Context, params json.RawMessage) (any, error) {
		if len(d.demo) > 0 {
			return nil, api.Errorf(api.CodeUnsupported, "demo mode does not update")
		}
		if !updateBusy.CompareAndSwap(false, true) {
			return nil, api.Errorf(api.CodeBusy, "an update is already running")
		}
		go d.applyUpdate(srv)
		return map[string]any{"state": "updating"}, nil
	})
}

func (d *daemon) applyUpdate(srv *api.Server) {
	publish := func(phase string, percent *float64, msg string) {
		ev := map[string]any{"phase": phase}
		if percent != nil {
			ev["percent"] = *percent
		}
		if msg != "" {
			ev["error"] = msg
		}
		srv.Publish("update.progress", ev)
	}
	fail := func(err error) {
		d.log.Error("update failed", "err", err)
		publish("error", nil, err.Error())
		updateBusy.Store(false)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	publish("checking", nil, "")
	info, err := d.updateChecker().Check(ctx, d.version, d.commit)
	if err != nil {
		fail(err)
		return
	}
	if !info.Available {
		fail(fmt.Errorf("already up to date"))
		return
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		fail(fmt.Errorf("updating from source needs the Go toolchain; install go and try again"))
		return
	}
	if appImagePath() != "" {
		d.installAppPayload(ctx, info, goTool, publish, fail)
		return
	}
	if err := writable(binDir()); err != nil {
		fail(fmt.Errorf("this install is not writable (%s); update it through your package manager", binDir()))
		return
	}
	d.sweepUpdateStages()
	stage, err := os.MkdirTemp(d.paths.DataDir, "update-*")
	if err != nil {
		fail(err)
		return
	}
	publish("downloading", nil, "")
	archivePath, sum, err := d.updateChecker().Source(ctx, info, stage, func(done int64) {
		publish("downloading", nil, "")
	})
	if err != nil {
		fail(err)
		return
	}
	d.log.Info("update source fetched", "commit", info.Latest, "sha256", sum)
	tree := filepath.Join(stage, "tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		fail(err)
		return
	}
	if err := archive.ExtractTgz(archivePath, tree); err != nil {
		fail(err)
		return
	}
	src, err := singleDir(tree)
	if err != nil {
		fail(err)
		return
	}
	root := filepath.Join(stage, "out")
	if err := os.MkdirAll(root, 0o755); err != nil {
		fail(err)
		return
	}
	publish("installing", nil, "")
	built, err := d.buildFromSource(ctx, goTool, src, root, info, publish)
	if err != nil {
		fail(err)
		return
	}
	updater := filepath.Join(binDir(), "hikari-update")
	if staged := filepath.Join(root, "hikari-update"); fileExists(staged) {
		if err := installExecutable(staged, updater); err != nil {
			fail(err)
			return
		}
	}
	if !fileExists(updater) {
		fail(fmt.Errorf("the source tree has no scripts/hikari-update"))
		return
	}
	publish("installing", nil, "")
	logFile, _ := os.OpenFile(filepath.Join(d.paths.DataDir, "update.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	var cmd *exec.Cmd
	if _, err := exec.LookPath("systemd-run"); err == nil {
		args := []string{"--user", "--unit=hikari-update", "--collect",
			"--setenv=HIKARI_UPDATE_FROM=" + d.version,
			"--setenv=HIKARI_UPDATE_COMMIT=" + info.Latest,
			"--setenv=HIKARI_INSTALL_DIR=" + binDir(),
			"--setenv=HIKARI_UPDATE_LOG=" + filepath.Join(d.paths.DataDir, "update.log")}
		for _, k := range []string{"WAYLAND_DISPLAY", "DISPLAY", "XDG_CURRENT_DESKTOP", "QT_QPA_PLATFORM", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
			if v := os.Getenv(k); v != "" {
				args = append(args, "--setenv="+k+"="+v)
			}
		}
		args = append(args, updater, root, built)
		cmd = exec.Command("systemd-run", args...)
	} else {
		cmd = exec.Command(updater, root, built)
		cmd.Env = append(os.Environ(), "HIKARI_UPDATE_FROM="+d.version,
			"HIKARI_UPDATE_COMMIT="+info.Latest, "HIKARI_INSTALL_DIR="+binDir())
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		fail(err)
		return
	}
	d.log.Info("update handed to the updater", "commit", info.Latest, "version", built, "pid", cmd.Process.Pid)
	go func() {
		time.Sleep(700 * time.Millisecond)
		_ = os.Remove(config.SocketPath())
		os.Exit(0)
	}()
}

// installAppPayload gives an AppImage its update without rewriting the image: the
// new build lands beside the data dir and AppRun runs it from there.
func (d *daemon) installAppPayload(ctx context.Context, info update.Info, goTool string, publish func(string, *float64, string), fail func(error)) {
	payload := d.appPayloadDir()
	if err := os.MkdirAll(filepath.Join(payload, "usr", "bin"), 0o755); err != nil {
		fail(err)
		return
	}
	stage, err := os.MkdirTemp(d.paths.DataDir, "update-*")
	if err != nil {
		fail(err)
		return
	}
	defer os.RemoveAll(stage)
	publish("downloading", nil, "")
	archivePath, sum, err := d.updateChecker().Source(ctx, info, stage, func(int64) {
		publish("downloading", nil, "")
	})
	if err != nil {
		fail(err)
		return
	}
	d.log.Info("update source fetched", "commit", info.Latest, "sha256", sum)
	tree := filepath.Join(stage, "tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		fail(err)
		return
	}
	if err := archive.ExtractTgz(archivePath, tree); err != nil {
		fail(err)
		return
	}
	src, err := singleDir(tree)
	if err != nil {
		fail(err)
		return
	}
	publish("installing", nil, "")
	version, err := d.buildInto(ctx, goTool, src, filepath.Join(payload, "usr", "bin"), info)
	if err != nil {
		fail(err)
		return
	}
	if !update.VersionAtLeast(version, d.version) {
		fail(fmt.Errorf("%s holds %s and this build is already %s; nothing to install", info.Latest, version, d.version))
		return
	}
	shell := filepath.Join(payload, "usr", "share", "hikari", "quickshell")
	if err := os.RemoveAll(shell); err != nil {
		fail(err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(shell), 0o755); err != nil {
		fail(err)
		return
	}
	if out, err := runQuiet(ctx, "cp", "-a", filepath.Join(src, "quickshell"), shell); err != nil {
		fail(fmt.Errorf("copying the shell files failed: %s", out))
		return
	}
	from := filepath.Join(appDirPath(), "usr", "share", "hikari", "amethyst_filegraph.abi3.so")
	if fileExists(from) {
		to := filepath.Join(payload, "usr", "share", "hikari", "amethyst_filegraph.abi3.so")
		if out, err := runQuiet(ctx, "cp", "-a", from, to); err != nil {
			fail(fmt.Errorf("copying the file catalog module failed: %s", out))
			return
		}
	}
	out, err := runQuiet(ctx, filepath.Join(payload, "usr", "bin", "hikarid"), "--version")
	if err != nil || !strings.Contains(out, info.Latest[:8]) {
		fail(fmt.Errorf("the new build does not run: %s", strings.TrimSpace(out)))
		return
	}
	if err := os.WriteFile(filepath.Join(payload, "commit"), []byte(info.Sha+"\n"), 0o644); err != nil {
		fail(err)
		return
	}
	d.log.Info("AppImage payload installed", "commit", info.Latest, "version", version, "path", payload)
	d.restartAppImage(appImagePath())
}

// appPayloadDir is where an AppImage keeps the build it updated itself to.
func (d *daemon) appPayloadDir() string {
	if dir := os.Getenv("HIKARI_APP_PAYLOAD"); dir != "" {
		return dir
	}
	return filepath.Join(filepath.Dir(d.paths.DataDir), "hikari", "app")
}

func appDirPath() string {
	for _, key := range []string{"HIKARI_APPIMAGE_DIR", "APPDIR"} {
		if dir := os.Getenv(key); dir != "" {
			return dir
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	bin := filepath.Dir(exe)
	if filepath.Base(bin) != "bin" {
		return ""
	}
	return filepath.Dir(filepath.Dir(bin))
}

func runQuiet(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// buildInto compiles both binaries from the fetched tree into dir and reports the
// version that tree carries.
func (d *daemon) buildInto(ctx context.Context, goTool, src, dir string, info update.Info) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, name := range []string{"hikarid", "hikari"} {
		out := filepath.Join(dir, name)
		args := []string{"build", "-trimpath", "-buildvcs=false",
			"-ldflags", "-s -w -X main.commit=" + info.Sha,
			"-o", out, "./cmd/" + name}
		cmd := exec.CommandContext(ctx, goTool, args...)
		cmd.Dir = src
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "CGO_ENABLED=0")
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("building %s from %s failed: %s", name, info.Latest, lastNonEmpty(stderr.String()))
		}
		if err := os.Chmod(out, 0o755); err != nil {
			return "", err
		}
	}
	return sourceVersion(filepath.Join(src, "cmd", "hikarid", "main.go"))
}

// buildFromSource turns a fetched tree into the layout the installer expects and
// returns the version the new daemon reports.
func (d *daemon) buildFromSource(ctx context.Context, goTool, src, root string, info update.Info, publish func(string, *float64, string)) (string, error) {
	if _, err := d.buildInto(ctx, goTool, src, filepath.Join(root, "bin"), info); err != nil {
		return "", err
	}
	for _, name := range []string{"hikarid", "hikari"} {
		if err := os.Rename(filepath.Join(root, "bin", name), filepath.Join(root, name)); err != nil {
			return "", err
		}
	}
	for _, item := range []struct{ from, to string }{
		{filepath.Join(src, "scripts", "hikari-app"), filepath.Join(root, "hikari-app")},
		{filepath.Join(src, "scripts", "hikari-update"), filepath.Join(root, "hikari-update")},
		{filepath.Join(src, "packaging", "hikari.desktop"), filepath.Join(root, "hikari.desktop")},
		{filepath.Join(src, "packaging", "hikari.svg"), filepath.Join(root, "hikari.svg")},
	} {
		if !fileExists(item.from) {
			return "", fmt.Errorf("the source tree is missing %s", filepath.Base(item.from))
		}
		content, err := os.ReadFile(item.from)
		if err != nil {
			return "", err
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(item.to, filepath.Join(root, "hikari-")) {
			mode = 0o755
		}
		if err := os.WriteFile(item.to, content, mode); err != nil {
			return "", err
		}
	}
	if err := copyTree(filepath.Join(src, "quickshell"), filepath.Join(root, "quickshell")); err != nil {
		return "", err
	}
	version, err := sourceVersion(filepath.Join(src, "cmd", "hikarid", "main.go"))
	if err != nil {
		return "", err
	}
	if !update.VersionAtLeast(version, d.version) {
		return "", fmt.Errorf("%s holds %s and this build is already %s; nothing to install", info.Latest, version, d.version)
	}
	return version, nil
}

func sourceVersion(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^\s*(?:var\s+)?version\s*=\s*"([^"]+)"`).FindSubmatch(raw)
	if len(m) != 2 {
		return "", fmt.Errorf("cannot read the version of the fetched source")
	}
	return string(m[1]), nil
}

func lastNonEmpty(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return "no output"
}

func (d *daemon) sweepUpdateStages() {
	entries, err := os.ReadDir(d.paths.DataDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "update-") && entry.IsDir() {
			_ = os.RemoveAll(filepath.Join(d.paths.DataDir, entry.Name()))
		}
	}
}

func (d *daemon) restartAppImage(appImage string) {
	script := `sleep 0.5
if [ -x "$1" ]; then "$1" -c hikari ipc call hikari close >/dev/null 2>&1 || true; fi
i=0
while [ -S "$2" ] && [ "$i" -lt 100 ]; do sleep 0.1; i=$((i+1)); done
systemctl --user stop hikarid-appimage >/dev/null 2>&1 || true
rm -f "$2"
exec "$3"`
	argv := []string{"/bin/sh", "-c", script, "sh", qsBin(), config.SocketPath(), appImage}
	var cmd *exec.Cmd
	if _, err := exec.LookPath("systemd-run"); err == nil {
		args := []string{"--user", "--unit=hikari-update", "--collect"}
		for _, k := range []string{"WAYLAND_DISPLAY", "DISPLAY", "XDG_CURRENT_DESKTOP", "QT_QPA_PLATFORM", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
			if v := os.Getenv(k); v != "" {
				args = append(args, "--setenv="+k+"="+v)
			}
		}
		args = append(args, argv...)
		cmd = exec.Command("systemd-run", args...)
	} else {
		cmd = exec.Command(argv[0], argv[1:]...)
		cmd.Env = os.Environ()
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	logFile, _ := os.OpenFile(filepath.Join(d.paths.DataDir, "update.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		d.log.Error("could not schedule the AppImage restart", "err", err)
		updateBusy.Store(false)
		return
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		_ = os.Remove(config.SocketPath())
		os.Exit(0)
	}()
}

func qsBin() string {
	if p, err := exec.LookPath("qs"); err == nil {
		return p
	}
	return "/usr/sbin/qs"
}

func installExecutable(src, dst string) error {
	body, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, body, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func writable(dir string) error {
	f, err := os.CreateTemp(dir, ".update-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func binDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

func singleDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if found != "" {
			return "", fmt.Errorf("the source archive has more than one top-level directory")
		}
		found = filepath.Join(dir, entry.Name())
	}
	if found == "" {
		return "", fmt.Errorf("the source archive is empty")
	}
	return found, nil
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(from, path)
		if relErr != nil {
			return relErr
		}
		dst := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(dst, body, info.Mode().Perm())
	})
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
