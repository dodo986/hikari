package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if appImagePath() != "" {
		c.Format = "appimage"
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

func qsBin() string {
	if p, err := exec.LookPath("qs"); err == nil {
		return p
	}
	return "/usr/sbin/qs"
}

func (d *daemon) registerUpdate(srv *api.Server) {
	srv.Register("app.update.check", func(c context.Context, params json.RawMessage) (any, error) {
		info, err := d.updateChecker().Check(c, d.version)
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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	publish("checking", nil, "")
	info, err := d.updateChecker().Check(ctx, d.version)
	if err != nil {
		fail(err)
		return
	}
	if !info.Available {
		fail(fmt.Errorf("already up to date"))
		return
	}
	if appImage := appImagePath(); appImage != "" {
		d.applyAppImageUpdate(ctx, info, appImage, publish, fail)
		return
	}
	if err := writable(binDir()); err != nil {
		fail(fmt.Errorf("this install is not writable (%s); update it through your package manager", binDir()))
		return
	}
	stage, err := os.MkdirTemp(d.paths.DataDir, "update-*")
	if err != nil {
		fail(err)
		return
	}
	publish("downloading", nil, "")
	archivePath, err := d.updateChecker().Download(ctx, info, stage, func(done, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(done) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
		publish("downloading", &pct, "")
	})
	if err != nil {
		fail(err)
		return
	}
	tree := filepath.Join(stage, "tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		fail(err)
		return
	}
	if err := archive.ExtractTgz(archivePath, tree); err != nil {
		fail(err)
		return
	}
	root, err := releaseRoot(tree)
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
		fail(fmt.Errorf("release has no hikari-update script"))
		return
	}
	publish("installing", nil, "")
	logFile, _ := os.OpenFile(filepath.Join(d.paths.DataDir, "update.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	// The updater must outlive this daemon's systemd unit: a child in the
	// same cgroup is killed when the unit's main process exits.
	var cmd *exec.Cmd
	if _, err := exec.LookPath("systemd-run"); err == nil {
		args := []string{"--user", "--unit=hikari-update", "--collect",
			"--setenv=HIKARI_UPDATE_FROM=" + d.version,
			"--setenv=HIKARI_INSTALL_DIR=" + binDir(),
			"--setenv=HIKARI_UPDATE_LOG=" + filepath.Join(d.paths.DataDir, "update.log")}
		for _, k := range []string{"WAYLAND_DISPLAY", "DISPLAY", "XDG_CURRENT_DESKTOP", "QT_QPA_PLATFORM", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
			if v := os.Getenv(k); v != "" {
				args = append(args, "--setenv="+k+"="+v)
			}
		}
		args = append(args, updater, root, info.Latest)
		cmd = exec.Command("systemd-run", args...)
	} else {
		cmd = exec.Command(updater, root, info.Latest)
		cmd.Env = append(os.Environ(), "HIKARI_UPDATE_FROM="+d.version, "HIKARI_INSTALL_DIR="+binDir())
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
	d.log.Info("update handed to the updater", "version", info.Latest, "pid", cmd.Process.Pid)
	go func() {
		time.Sleep(700 * time.Millisecond)
		_ = os.Remove(config.SocketPath())
		os.Exit(0)
	}()
}

func (d *daemon) applyAppImageUpdate(ctx context.Context, info update.Info, appImage string, publish func(string, *float64, string), fail func(error)) {
	if resolved, err := filepath.EvalSymlinks(appImage); err == nil {
		appImage = resolved
	}
	dir := filepath.Dir(appImage)
	if err := writable(dir); err != nil {
		fail(fmt.Errorf("the AppImage folder is not writable (%s); move Hikari somewhere writable or download the new release by hand", dir))
		return
	}
	stage, err := os.MkdirTemp(dir, ".hikari-update-*")
	if err != nil {
		fail(err)
		return
	}
	defer os.RemoveAll(stage)
	publish("downloading", nil, "")
	asset, err := d.updateChecker().Download(ctx, info, stage, func(done, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(done) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
		publish("downloading", &pct, "")
	})
	if err != nil {
		fail(err)
		return
	}
	if err := os.Chmod(asset, 0o755); err != nil {
		fail(err)
		return
	}
	publish("installing", nil, "")
	if err := os.Rename(asset, appImage); err != nil {
		fail(err)
		return
	}
	d.log.Info("AppImage replaced, restarting", "version", info.Latest, "path", appImage)
	d.restartAppImage(appImage)
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

func releaseRoot(dir string) (string, error) {
	if fileExists(filepath.Join(dir, "hikarid")) {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if fileExists(filepath.Join(dir, e.Name(), "hikarid")) {
			if found != "" {
				return "", fmt.Errorf("release has more than one top-level directory")
			}
			found = filepath.Join(dir, e.Name())
		}
	}
	if found == "" {
		return "", fmt.Errorf("release has no hikarid binary")
	}
	for _, name := range []string{"hikari", "hikari-app", "hikari-update", "quickshell/shell.qml"} {
		if !fileExists(filepath.Join(found, name)) {
			return "", fmt.Errorf("release is missing %s", name)
		}
	}
	return found, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
