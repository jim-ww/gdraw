// Command gdraw is a minimalist canvas for quick drawings.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
)

// version is set at release build time; see .goreleaser.yaml.
var version = ""

func main() {
	restore := flag.Bool("restore", false, "reopen unsaved drawings recovered from an earlier session")
	showVersion := flag.Bool("version", false, "print the version and exit")
	configPath := flag.String("c", "", "config file (default: config.json in the user config dir)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: gdraw [-c config.json] [-restore] [-version] [file.json ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("gdraw", buildVersion())
		return
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gdraw: config:", err)
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("gdraw"), app.Size(unit.Dp(1200), unit.Dp(800)))
		u := newUI(cfg)
		if *restore {
			u.restore()
		}
		for _, path := range flag.Args() {
			u.openPath(path)
		}
		if err := run(w, u); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

// buildVersion returns the release version, falling back to the module
// version recorded by go install.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

func run(w *app.Window, u *UI) error {
	var ops op.Ops
	var title string
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			// Closing the window can't be vetoed, so unsaved work goes to
			// recovery files instead.
			u.autosave()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			u.Layout(gtx)
			e.Frame(gtx.Ops)
			// Window methods wait for the native event loop, which is
			// blocked during a frame, so they run only after it.
			if t := u.windowTitle(); t != title {
				title = t
				w.Option(app.Title(t))
			}
			if u.quitting {
				w.Perform(system.ActionClose)
			}
		}
	}
}
