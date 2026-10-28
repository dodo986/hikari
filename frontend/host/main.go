package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

func main() {
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
