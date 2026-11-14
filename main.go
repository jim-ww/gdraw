// Command gdraw is a minimalist canvas for quick drawings.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
)

func main() {
	restore := flag.Bool("restore", false, "reopen unsaved drawings recovered from an earlier session")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: gdraw [-restore] [file.json ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("gdraw"), app.Size(unit.Dp(1200), unit.Dp(800)))
		u := newUI(w)
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

func run(w *app.Window, u *UI) error {
	var ops op.Ops
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
		}
	}
}
