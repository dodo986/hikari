package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Host is what a browser cannot do itself. Everything else the page does over
// the websocket, so the same bundle runs outside this window.
type Host struct {
	ctx context.Context
}

func newHost() *Host { return &Host{} }

func (h *Host) startup(ctx context.Context) {
	h.ctx = ctx
	go h.watchPalette()
}

// Palette answers with the current roles, for a page that subscribed after the
// window had already repainted.
func (h *Host) Palette() any { return readPalette() }

func (h *Host) Daemon() (Endpoint, error) { return readEndpoint() }

func (h *Host) PickFile(label string) (string, error) {
	if label == "" {
		label = "Choose a file"
	}
	return runtime.OpenFileDialog(h.ctx, runtime.OpenDialogOptions{Title: label})
}

func (h *Host) PickFolder(label string) (string, error) {
	if label == "" {
		label = "Choose a folder"
	}
	return runtime.OpenDirectoryDialog(h.ctx, runtime.OpenDialogOptions{Title: label})
}

func (h *Host) OpenUrl(url string) error {
	if url == "" {
		return errors.New("no url")
	}
	runtime.BrowserOpenURL(h.ctx, url)
	return nil
}

// StartDaemon starts hikarid. A build tree keeps the daemon beside the window,
// so that copy wins: it is the one this frontend was built against. Otherwise
// the user service, then whatever is on PATH.
func (h *Host) StartDaemon() error {
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "hikarid")
		if info, statErr := os.Stat(sibling); statErr == nil && !info.IsDir() {
			return spawn(sibling)
		}
	}
	if err := exec.Command("systemctl", "--user", "start", "hikarid").Run(); err == nil {
		return nil
	}
	path, err := exec.LookPath("hikarid")
	if err != nil {
		return errors.New("hikarid is not installed and the user service could not start it")
	}
	return spawn(path)
}

// pendingLink is the nxm:// link the desktop handed to this window, if any.
func pendingLink() string {
	return linkIn(os.Args[1:])
}

func linkIn(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "nxm://") {
			return arg
		}
	}
	return ""
}

func spawn(path string) error {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
