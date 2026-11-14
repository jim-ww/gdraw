package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	barBg     = color.NRGBA{0x1c, 0x1c, 0x1c, 0xff}
	btnHover  = color.NRGBA{0x2e, 0x2e, 0x2e, 0xff}
	btnActive = color.NRGBA{0x45, 0x45, 0x45, 0xff}
	textCol   = color.NRGBA{0xd8, 0xd8, 0xd8, 0xff}
	dimCol    = color.NRGBA{0x80, 0x80, 0x80, 0xff}
)

var defaultPalette = []string{
	"#ffffff", "#ff0000", "#00ff00", "#0000ff",
	"#ffff00", "#ff00ff", "#00ffff", "#808080",
	"#000000",
}

// UI is the whole application state.
type UI struct {
	win    *app.Window
	th     *material.Theme
	keymap *keymap
	render renderer
	scene  scene

	tabs    []*Document
	active  int
	tool    Tool
	color   string
	size    float64
	filled  bool
	grid    bool
	snap    bool
	palette []string
	recent  []string
	status  string
	tip     string
	title   string
	help    bool

	nextAutosave time.Time

	drag      drag
	pan       panState
	prompt    *prompt
	typing    textEdit
	spaceDown bool
	hover     *Edit
	clipboard []*Edit

	cursor     f32.Point // canvas pixels
	cursorIn   bool
	canvasSize image.Point
	pxPerDp    float32

	btn struct {
		tools    [numTools]widget.Clickable
		swatches []widget.Clickable
		tabs     []tabButtons
		newTab   widget.Clickable
		sizeDec  widget.Clickable
		sizeInc  widget.Clickable
		color    widget.Clickable
		filled   widget.Clickable
		grid     widget.Clickable
		snap     widget.Clickable
	}
}

type tabButtons struct{ open, close widget.Clickable }

type panState struct {
	active bool
	last   f32.Point
}

func newUI(w *app.Window, cfg Config) *UI {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	th.Palette = material.Palette{Bg: barBg, Fg: textCol, ContrastBg: btnActive, ContrastFg: textCol}
	th.TextSize = 13
	km, errs := newKeymap(defaultActions(), cfg.Keys)
	u := &UI{
		win:     w,
		th:      th,
		keymap:  km,
		render:  renderer{shaper: th.Shaper},
		tabs:    []*Document{NewDocument()},
		color:   defaultPalette[0],
		size:    3,
		grid:    true,
		palette: defaultPalette,
		recent:  loadRecent(),
		pxPerDp: 1,
	}
	if len(cfg.Palette) > 0 {
		u.palette = cfg.Palette
		u.color = cfg.Palette[0]
	}
	if cfg.DefaultColor != "" {
		u.color = cfg.DefaultColor
	}
	if cfg.DefaultSize > 0 {
		u.setSize(cfg.DefaultSize)
	}
	u.grid = !cfg.HideGrid
	if k := km.keysFor("help"); k != "" {
		u.status = "press " + k + " for keys"
	}
	if n := len(recoveryFiles()); n > 0 {
		u.status = fmt.Sprintf("found %s from an earlier session; run gdraw -restore to recover", plural(n, "unsaved drawing"))
	}
	u.btn.swatches = make([]widget.Clickable, len(u.palette))
	if len(errs) > 0 {
		u.status = fmt.Sprintf("config keys: %v", errors.Join(errs...))
	}
	return u
}

func (u *UI) Layout(gtx C) D {
	u.pxPerDp = gtx.Metric.PxPerDp
	u.tip = ""
	if gtx.Now.After(u.nextAutosave) {
		if !u.nextAutosave.IsZero() {
			u.autosave()
		}
		u.nextAutosave = gtx.Now.Add(autosaveInterval)
	}
	gtx.Execute(op.InvalidateCmd{At: u.nextAutosave})
	u.handleKeys(gtx)

	paint.Fill(gtx.Ops, barBg)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.layoutTabs),
		layout.Rigid(u.layoutToolbar),
		layout.Flexed(1, func(gtx C) D {
			dims := u.layoutCanvas(gtx)
			if u.help {
				u.layoutHelp(gtx)
			}
			return dims
		}),
		layout.Rigid(u.layoutPrompt),
		layout.Rigid(u.layoutStatus),
	)
	if t := "gdraw - " + u.doc().Title(); t != u.title {
		u.title = t
		u.win.Option(app.Title(t))
	}
	return dims
}

func (u *UI) layoutTabs(gtx C) D {
	for len(u.btn.tabs) < len(u.tabs) {
		u.btn.tabs = append(u.btn.tabs, tabButtons{})
	}
	var items []layout.Widget
	for i, d := range u.tabs {
		b := &u.btn.tabs[i]
		if b.close.Clicked(gtx) {
			u.requestCloseTab(i)
			gtx.Execute(op.InvalidateCmd{})
			return D{}
		}
		if b.open.Clicked(gtx) {
			u.selectTab(i)
		}
		items = append(items, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return u.button(gtx, &b.open, d.Title(), i == u.active, "") }),
				layout.Rigid(func(gtx C) D { return u.button(gtx, &b.close, "×", false, "close tab") }),
			)
		})
	}
	if u.btn.newTab.Clicked(gtx) {
		u.newTab()
	}
	items = append(items, func(gtx C) D {
		return u.button(gtx, &u.btn.newTab, "+", false, u.withKey("new tab", "new_tab"))
	})
	return flow(gtx, items...)
}

func (u *UI) layoutToolbar(gtx C) D {
	var items []layout.Widget
	for t := range numTools {
		b := &u.btn.tools[t]
		if b.Clicked(gtx) {
			u.setTool(t)
		}
		items = append(items, func(gtx C) D {
			return u.button(gtx, b, t.String(), u.tool == t, u.withKey(t.String(), "tool_"+t.String()))
		})
	}
	items = append(items, spacer)
	for i, c := range u.palette {
		b := &u.btn.swatches[i]
		if b.Clicked(gtx) {
			u.color = c
		}
		items = append(items, func(gtx C) D { return u.swatch(gtx, b, c) })
	}
	if u.btn.color.Clicked(gtx) {
		u.pickColor()
	}
	items = append(items,
		func(gtx C) D {
			return u.button(gtx, &u.btn.color, u.color, false, u.withKey("pick color", "pick_color"))
		},
		spacer,
	)

	if u.btn.sizeDec.Clicked(gtx) {
		u.setSize(u.size / sizeStep)
	}
	if u.btn.sizeInc.Clicked(gtx) {
		u.setSize(u.size * sizeStep)
	}
	toggle := func(b *widget.Clickable, v *bool, label, act string) layout.Widget {
		if b.Clicked(gtx) {
			*v = !*v
		}
		return func(gtx C) D { return u.button(gtx, b, label, *v, u.withKey("toggle "+label, act)) }
	}
	items = append(items,
		func(gtx C) D { return u.button(gtx, &u.btn.sizeDec, "−", false, u.withKey("size down", "size_dec")) },
		func(gtx C) D { return u.label(gtx, fmt.Sprintf("%g", u.size), textCol) },
		func(gtx C) D { return u.button(gtx, &u.btn.sizeInc, "+", false, u.withKey("size up", "size_inc")) },
		spacer,
		toggle(&u.btn.filled, &u.filled, "filled", "toggle_fill"),
		toggle(&u.btn.grid, &u.grid, "grid", "toggle_grid"),
		toggle(&u.btn.snap, &u.snap, "snap", "toggle_snap"),
	)
	return flow(gtx, items...)
}

func (u *UI) layoutStatus(gtx C) D {
	msg := u.status
	if u.tip != "" {
		msg = u.tip
	}
	info := fmt.Sprintf("%s · %.0f%%", u.tool, u.doc().Zoom*100)
	return layout.Inset{Left: 8, Right: 8, Top: 3, Bottom: 3}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return u.text(gtx, msg, dimCol) }),
			layout.Rigid(func(gtx C) D { return u.text(gtx, info, dimCol) }),
		)
	})
}

// withKey formats a tooltip with the key bound to act.
func (u *UI) withKey(tip, act string) string {
	if k := u.keymap.keysFor(act); k != "" {
		return fmt.Sprintf("%s (%s)", tip, k)
	}
	return tip
}

func (u *UI) button(gtx C, b *widget.Clickable, label string, active bool, tip string) D {
	if b.Hovered() && tip != "" {
		u.tip = tip
	}
	return b.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				bg := barBg
				switch {
				case active:
					bg = btnActive
				case b.Hovered():
					bg = btnHover
				}
				paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(3)).Op(gtx.Ops))
				return D{Size: gtx.Constraints.Min}
			},
			func(gtx C) D { return u.label(gtx, label, textCol) },
		)
	})
}

func (u *UI) label(gtx C, s string, c color.NRGBA) D {
	return layout.Inset{Left: 7, Right: 7, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D { return u.text(gtx, s, c) })
}

func (u *UI) text(gtx C, s string, c color.NRGBA) D {
	l := material.Body2(u.th, s)
	l.Color = c
	l.MaxLines = 1
	return l.Layout(gtx)
}

func (u *UI) swatch(gtx C, b *widget.Clickable, hex string) D {
	if b.Hovered() {
		u.tip = hex
	}
	return layout.UniformInset(3).Layout(gtx, func(gtx C) D {
		return b.Layout(gtx, func(gtx C) D {
			sz := gtx.Dp(18)
			r := image.Rectangle{Max: image.Pt(sz, sz)}
			paint.FillShape(gtx.Ops, parseColor(hex), clip.UniformRRect(r, gtx.Dp(3)).Op(gtx.Ops))
			if hex == u.color || b.Hovered() {
				c := dimCol
				if hex == u.color {
					c = textCol
				}
				paint.FillShape(gtx.Ops, c, clip.Stroke{Path: clip.UniformRRect(r, gtx.Dp(3)).Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
			}
			return D{Size: r.Max}
		})
	})
}

func spacer(gtx C) D { return D{Size: image.Pt(gtx.Dp(10), 0)} }

// flow lays out widgets left to right, wrapping to new rows as needed.
func flow(gtx C, ws ...layout.Widget) D {
	gtx.Constraints.Min = image.Point{}
	maxW := gtx.Constraints.Max.X
	var x, y, rowH int
	for _, w := range ws {
		m := op.Record(gtx.Ops)
		dims := w(gtx)
		call := m.Stop()
		if x > 0 && x+dims.Size.X > maxW {
			x, y, rowH = 0, y+rowH, 0
		}
		t := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		t.Pop()
		x += dims.Size.X
		rowH = max(rowH, dims.Size.Y)
	}
	return D{Size: image.Pt(maxW, y+rowH)}
}
