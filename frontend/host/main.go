package main

import (
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// kept open for the process lifetime so a second launch sees it taken.
var instanceLock *os.File

// singleInstance answers whether this process owns the window slot, so a
// second click on the launcher exits instead of opening a duplicate window.
func singleInstance() bool {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	file, err := os.OpenFile(filepath.Join(dir, "hikari-ui.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return false
	}
	instanceLock = file
	return true
}

func main() {
	if !singleInstance() {
		log.Print("hikari: the window is already open")
		return
	}
	host := newHost()
	err := wails.Run(&options.App{
		Title:            "Hikari",
		Width:            1400,
		Height:           900,
		MinWidth:         1000,
		MinHeight:        700,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		AssetServer:      &assetserver.Options{Handler: assetHandler()},
		OnStartup:        host.startup,
		Bind:             []interface{}{host},
		Linux: &linux.Options{
			ProgramName: "hikari",
		},
	})
	if err != nil {
		log.Fatalf("hikari: the window could not start: %v", err)
	}
}
