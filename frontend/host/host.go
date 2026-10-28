package main

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Host is what a browser cannot do itself. Everything else the page does over
// the websocket, so the same bundle runs outside this window.
type Host struct {
	ctx context.Context
}

func newHost() *Host { return &Host{} }

func (h *Host) startup(ctx context.Context) { h.ctx = ctx }

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

// StartDaemon starts hikarid as a user service, which is the only thing the
// offline screen can do. It never waits: the page reconnects by itself.
func (h *Host) StartDaemon() error {
	if err := exec.Command("systemctl", "--user", "start", "hikarid").Run(); err == nil {
		return nil
	}
	path, err := exec.LookPath("hikarid")
	if err != nil {
		return errors.New("hikarid is not installed and the user service could not start it")
	}
	cmd := exec.Command(path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
