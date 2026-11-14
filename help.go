package main

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

var helpBg = color.NRGBA{0x12, 0x12, 0x12, 0xf0}

var mouseHelp = [][2]string{
	{"wheel", "zoom at cursor"},
	{"middle drag", "pan"},
	{"space+drag", "pan"},
	{"ctrl+drag", "45° lines, square shapes"},
}

func (u *UI) handleHelpKeys(gtx C) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameF1},
			key.Filter{Name: "?", Optional: key.ModShift},
		)
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			u.help = false
		}
	}
}

// layoutHelp draws the keybinding reference over the canvas.
func (u *UI) layoutHelp(gtx C) D {
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: helpBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	// Swallow pointer input so clicks don't draw underneath.
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: &u.help, Kinds: pointer.Press | pointer.Release}); !ok {
			break
		}
		u.help = false
	}
	pointer.CursorDefault.Add(gtx.Ops)
	event.Op(gtx.Ops, &u.help)

	var items []layout.Widget
	entry := func(k, desc string) layout.Widget {
		return func(gtx C) D {
			return layout.Inset{Right: 24, Bottom: 2}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(240)
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Dp(110)
						return u.text(gtx, k, textCol)
					}),
					layout.Rigid(func(gtx C) D { return u.text(gtx, desc, dimCol) }),
				)
			})
		}
	}
	for _, m := range mouseHelp {
		items = append(items, entry(m[0], m[1]))
	}
	for i, a := range u.keymap.actions {
		for j, b := range u.keymap.bindings[i] {
			if j > 0 {
				break
			}
			items = append(items, entry(b.String(), a.help))
		}
	}
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		return flow(gtx, items...)
	})
}
