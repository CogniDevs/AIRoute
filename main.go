package main

import (
	"embed"
	"flag"
	"fmt"
	"os"

	"airoute/internal/hosts"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var applyHostsPath string
	var cleanHosts bool

	flag.StringVar(&applyHostsPath, "apply-hosts", "", "Apply temporary file directly to system hosts and exit")
	flag.BoolVar(&cleanHosts, "clean-hosts", false, "Clean AIRoute rules from system hosts and exit")
	flag.Parse()

	// Elevated or CLI helper mode: apply hosts directly
	if applyHostsPath != "" {
		data, err := os.ReadFile(applyHostsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read temp hosts file %s: %v\n", applyHostsPath, err)
			os.Exit(1)
		}
		mgr := hosts.NewManager()
		if err := os.WriteFile(mgr.GetHostsPath(), data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write system hosts: %v\n", err)
			os.Exit(1)
		}
		_ = hosts.FlushDNS()
		os.Exit(0)
	}

	// Elevated or CLI helper mode: clean hosts directly
	if cleanHosts {
		mgr := hosts.NewManager()
		if err := mgr.ClearRoute(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean hosts: %v\n", err)
			os.Exit(1)
		}
		_ = hosts.FlushDNS()
		os.Exit(0)
	}

	// Normal Wails GUI startup
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "AIRoute — Smart AI Hosts Patcher",
		Width:     840,
		Height:    740,
		MinWidth:  680,
		MinHeight: 580,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
