package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
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
	activeCol = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	tipBg     = color.NRGBA{0x2a, 0x2a, 0x2a, 0xf8}
)

var defaultPalette = []string{
	"#ffffff", "#ff0000", "#00ff00", "#0000ff",
	"#ffff00", "#ff00ff", "#00ffff", "#808080",
	"#000000",
}

// UI is the whole application state.
type UI struct {
	th        *material.Theme
	keymap    *keymap
	render    renderer
	scene     scene
	gridCache grid

	tabs     []*Document
	active   int
	tool     Tool
	color    string
	size     float64
	filled   bool
	grid     bool
	snap     bool
	palette  []string
	recent   []string
	status   string
	tip      string    // tooltip of the hovered control
	tipFrom  time.Time // when the pointer came to rest on it
	shown    string    // tooltip text tipFrom refers to
	mouse    f32.Point // pointer position in the window
	quitting bool      // close the window after this frame
	help     bool

	nextAutosave time.Time

	sizeField numField
	zoomField numField

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
		actions  map[string]*widget.Clickable
		color    widget.Clickable
	}
}

type tabButtons struct{ open, close widget.Clickable }

type panState struct {
	active bool
	last   f32.Point
}

func newUI(cfg Config) *UI {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	th.Palette = material.Palette{Bg: barBg, Fg: textCol, ContrastBg: btnActive, ContrastFg: textCol}
	th.TextSize = 14
	km, errs := newKeymap(defaultActions(), cfg.Keys)
	u := &UI{
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
	u.btn.actions = make(map[string]*widget.Clickable)
	if len(errs) > 0 {
		u.status = fmt.Sprintf("config keys: %v", errors.Join(errs...))
	}
	return u
}

// tipDelay is how long the pointer rests on a control before its tooltip shows.
const tipDelay = 400 * time.Millisecond

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
	u.trackPointer(gtx)
	u.layoutTooltip(gtx)
	return dims
}

// windowTitle is the title the window should show.
func (u *UI) windowTitle() string { return "gdraw - " + u.doc().Title() }

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
			return u.button(gtx, &b.open, i == u.active, d.Path, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Left: 10, Top: 7, Bottom: 7, Right: 4}.Layout(gtx, func(gtx C) D {
							return u.text(gtx, d.Title(), textCol)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Right: 4}.Layout(gtx, func(gtx C) D {
							return u.iconButton(gtx, &b.close, closeIcon, 14, false, "close tab")
						})
					}),
				)
			})
		})
	}
	items = append(items, u.actionButton(gtx, "new_tab"))
	return layout.Inset{Left: 4, Right: 4, Top: 4}.Layout(gtx, func(gtx C) D { return flow(gtx, items...) })
}

func (u *UI) layoutToolbar(gtx C) D {
	items := []layout.Widget{
		u.actionButton(gtx, "open"),
		u.actionButton(gtx, "save"),
		u.actionButton(gtx, "export"),
		separator,
		u.actionButton(gtx, "undo"),
		u.actionButton(gtx, "redo"),
		separator,
	}
	for t := range numTools {
		b := &u.btn.tools[t]
		if b.Clicked(gtx) {
			u.setTool(t)
		}
		items = append(items, func(gtx C) D {
			return u.iconButton(gtx, b, toolIcons[t], 20, u.tool == t, u.withKey(t.String(), "tool_"+t.String()))
		})
	}
	items = append(items, separator)
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
	const numTip = "scroll, drag sideways or click to type"
	items = append(items,
		func(gtx C) D { return u.colorButton(gtx) },
		separator,
		u.actionButton(gtx, "size_dec"),
		func(gtx C) D {
			return u.layoutNumField(gtx, &u.sizeField, numFieldSpec{
				label: "size", value: u.size, tip: "brush size: " + numTip,
				step: func(n int) { u.setSize(u.size * math.Pow(sizeStep, float64(n))) },
				set:  u.setSize,
			})
		},
		u.actionButton(gtx, "size_inc"),
		separator,
		u.actionButton(gtx, "zoom_out"),
		func(gtx C) D {
			return u.layoutNumField(gtx, &u.zoomField, numFieldSpec{
				label: "zoom", value: math.Round(u.doc().Zoom * 100), unit: "%", tip: "zoom: " + numTip,
				step: func(n int) { u.zoomAtCursor(math.Pow(zoomStep, float64(n))) },
				set:  func(v float64) { u.zoomAtCursor(v / 100 / u.doc().Zoom) },
			})
		},
		u.actionButton(gtx, "zoom_in"),
		separator,
		u.toggleButton(gtx, "toggle_fill", u.filled),
		u.toggleButton(gtx, "toggle_grid", u.grid),
		u.toggleButton(gtx, "toggle_snap", u.snap),
		separator,
		u.actionButton(gtx, "help"),
	)
	return layout.Inset{Left: 4, Right: 4, Top: 2, Bottom: 4}.Layout(gtx, func(gtx C) D { return flow(gtx, items...) })
}

// actionButton returns a toolbar button that runs the named action.
func (u *UI) actionButton(gtx C, name string) layout.Widget {
	return u.actionToggle(gtx, name, false)
}

// toggleButton is an actionButton highlighted while on is true.
func (u *UI) toggleButton(gtx C, name string, on bool) layout.Widget {
	return u.actionToggle(gtx, name, on)
}

func (u *UI) actionToggle(gtx C, name string, active bool) layout.Widget {
	b := u.btn.actions[name]
	if b == nil {
		b = new(widget.Clickable)
		u.btn.actions[name] = b
	}
	a, _ := u.keymap.action(name)
	if b.Clicked(gtx) && a.run != nil {
		a.run(u)
	}
	return func(gtx C) D { return u.iconButton(gtx, b, actionIcons[name], 20, active, u.withKey(a.help, name)) }
}

// colorButton shows the current color; clicking it asks for a hex value.
func (u *UI) colorButton(gtx C) D {
	return u.button(gtx, &u.btn.color, false, u.withKey("pick color", "pick_color"), func(gtx C) D {
		return layout.Inset{Left: 8, Right: 10, Top: 7, Bottom: 7}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					sz := gtx.Dp(14)
					paint.FillShape(gtx.Ops, parseColor(u.color), clip.Ellipse{Max: image.Pt(sz, sz)}.Op(gtx.Ops))
					return D{Size: image.Pt(sz+gtx.Dp(6), sz)}
				}),
				layout.Rigid(func(gtx C) D { return u.text(gtx, u.color, textCol) }),
			)
		})
	})
}

func (u *UI) layoutStatus(gtx C) D {
	info := fmt.Sprintf("%s · %.0f%%", u.tool, u.doc().Zoom*100)
	return layout.Inset{Left: 10, Right: 10, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return u.text(gtx, u.status, dimCol) }),
			layout.Rigid(func(gtx C) D { return u.text(gtx, info, dimCol) }),
		)
	})
}

// trackPointer follows the pointer across the window without taking its
// events from the controls underneath.
func (u *UI) trackPointer(gtx C) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &u.mouse, Kinds: pointer.Move | pointer.Drag | pointer.Enter})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			u.mouse = e.Position
		}
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &u.mouse)
}

// layoutTooltip shows the hovered control's tooltip near the pointer once
// it has rested there for tipDelay.
func (u *UI) layoutTooltip(gtx C) {
	if u.tip != u.shown {
		u.shown, u.tipFrom = u.tip, gtx.Now
	}
	if u.tip == "" {
		return
	}
	if wait := u.tipFrom.Add(tipDelay); gtx.Now.Before(wait) {
		gtx.Execute(op.InvalidateCmd{At: wait})
		return
	}
	m := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := layout.Inset{Left: 8, Right: 8, Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D { return u.text(gtx, u.tip, textCol) })
	call := m.Stop()

	win := gtx.Constraints.Max
	pos := u.mouse.Round().Add(image.Pt(gtx.Dp(12), gtx.Dp(20)))
	pos.X = max(0, min(pos.X, win.X-dims.Size.X))
	if pos.Y+dims.Size.Y > win.Y {
		pos.Y = u.mouse.Round().Y - dims.Size.Y - gtx.Dp(8)
	}
	defer op.Offset(pos).Push(gtx.Ops).Pop()
	r := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(4))
	paint.FillShape(gtx.Ops, tipBg, r.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, btnActive, clip.Stroke{Path: r.Path(gtx.Ops), Width: 1}.Op())
	call.Add(gtx.Ops)
}

// withKey formats a tooltip with the key bound to act.
func (u *UI) withKey(tip, act string) string {
	if k := u.keymap.keysFor(act); k != "" {
		return fmt.Sprintf("%s (%s)", tip, k)
	}
	return tip
}

// button draws content as a flat button, highlighted when active or hovered.
func (u *UI) button(gtx C, b *widget.Clickable, active bool, tip string, content layout.Widget) D {
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
				paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, gtx.Dp(4)).Op(gtx.Ops))
				return D{Size: gtx.Constraints.Min}
			},
			content,
		)
	})
}

// iconButton is a button showing ic at size dp.
func (u *UI) iconButton(gtx C, b *widget.Clickable, ic icon, size unit.Dp, active bool, tip string) D {
	return u.button(gtx, b, active, tip, func(gtx C) D {
		return layout.UniformInset(7).Layout(gtx, func(gtx C) D {
			sz := gtx.Dp(size)
			col := textCol
			if active {
				col = activeCol
			}
			ic(gtx, sz, col)
			return D{Size: image.Pt(sz, sz)}
		})
	})
}

// textButton is a button showing label.
func (u *UI) textButton(gtx C, b *widget.Clickable, label string, active bool, tip string) D {
	return u.button(gtx, b, active, tip, func(gtx C) D {
		return layout.Inset{Left: 10, Right: 10, Top: 7, Bottom: 7}.Layout(gtx, func(gtx C) D {
			return u.text(gtx, label, textCol)
		})
	})
}

// label is padded text, sized like a button.
func (u *UI) label(gtx C, s string, c color.NRGBA) D {
	return layout.Inset{Left: 10, Right: 10, Top: 7, Bottom: 7}.Layout(gtx, func(gtx C) D { return u.text(gtx, s, c) })
}

func (u *UI) text(gtx C, s string, c color.NRGBA) D {
	l := material.Body1(u.th, s)
	l.Color = c
	l.MaxLines = 1
	return l.Layout(gtx)
}

func (u *UI) swatch(gtx C, b *widget.Clickable, hex string) D {
	if b.Hovered() {
		u.tip = hex
	}
	return b.Layout(gtx, func(gtx C) D {
		return layout.UniformInset(6).Layout(gtx, func(gtx C) D {
			sz := gtx.Dp(22)
			r := image.Rectangle{Max: image.Pt(sz, sz)}
			rr := clip.UniformRRect(r, gtx.Dp(4))
			paint.FillShape(gtx.Ops, parseColor(hex), rr.Op(gtx.Ops))
			if hex == u.color || b.Hovered() {
				c := dimCol
				if hex == u.color {
					c = activeCol
				}
				paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
			}
			return D{Size: r.Max}
		})
	})
}

// separator is a thin vertical rule between toolbar groups.
func separator(gtx C) D {
	w, h := gtx.Dp(13), gtx.Dp(34)
	x, pad := gtx.Dp(6), gtx.Dp(8)
	paint.FillShape(gtx.Ops, btnHover, clip.Rect{Min: image.Pt(x, pad), Max: image.Pt(x+max(1, gtx.Dp(1)), h-pad)}.Op())
	return D{Size: image.Pt(w, h)}
}

// flow lays out widgets left to right, wrapping to new rows as needed and
// centering each widget vertically in its row.
func flow(gtx C, ws ...layout.Widget) D {
	gtx.Constraints.Min = image.Point{}
	maxW := gtx.Constraints.Max.X
	type item struct {
		call op.CallOp
		pos  image.Point
		size image.Point
		row  int
	}
	items := make([]item, len(ws))
	var rowH []int
	x, y := 0, 0
	for i, w := range ws {
		m := op.Record(gtx.Ops)
		dims := w(gtx)
		items[i].call, items[i].size = m.Stop(), dims.Size
		if x > 0 && x+dims.Size.X > maxW {
			x, y = 0, y+rowH[len(rowH)-1]
		}
		if x == 0 {
			rowH = append(rowH, 0)
		}
		items[i].pos, items[i].row = image.Pt(x, y), len(rowH)-1
		x += dims.Size.X
		rowH[len(rowH)-1] = max(rowH[len(rowH)-1], dims.Size.Y)
	}
	for _, it := range items {
		t := op.Offset(it.pos.Add(image.Pt(0, (rowH[it.row]-it.size.Y)/2))).Push(gtx.Ops)
		it.call.Add(gtx.Ops)
		t.Pop()
	}
	if len(rowH) == 0 {
		return D{Size: image.Pt(maxW, 0)}
	}
	return D{Size: image.Pt(maxW, y+rowH[len(rowH)-1])}
}
