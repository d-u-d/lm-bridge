package main

import (
	"context"
	"embed"
	"os"

	"lm-bridge/internal/cli"
	"lm-bridge/internal/db"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is set at build time via -ldflags "-X main.Version=v1.2.3"
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		cli.Run(os.Args[1:])
		return
	}

	store, err := db.Open()
	if err != nil {
		println("db error:", err.Error())
		os.Exit(1)
	}

	app := NewApp(store)

	if err := wails.Run(&options.App{
		Title:            "LM Bridge",
		Width:            860,
		Height:           580,
		MinWidth:         720,
		MinHeight:        440,
		StartHidden:      true, // tray click reveals the window
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 18, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		// Hide window on close — tray keeps the app alive
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			runtime.WindowHide(ctx)
			return true // prevent actual close
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "LM Bridge",
				Message: "© 2025 — Local LLM helper for Claude Code",
			},
		},
	}); err != nil {
		println("Error:", err.Error())
	}
}
