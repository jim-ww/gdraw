package main

import (
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// numField is a toolbar number: scroll over it or drag sideways to step
// it, click it to type a value.
type numField struct {
	hovered bool
	pressed bool
	dragged bool
	lastX   float32
	scrub   float32 // horizontal drag not yet turned into steps
	scroll  float32 // small scroll deltas not yet turned into steps

	editing bool
	focused bool
	ed      widget.Editor
}

// numFieldSpec says what a numField shows and how it changes its value.
type numFieldSpec struct {
	label string
	value float64
	unit  string
	tip   string
	step  func(n int)     // step by n increments
	set   func(v float64) // set a typed value
}

func (f *numField) events(gtx C, s numFieldSpec) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  f,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll | pointer.Enter | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: math.MinInt32, Max: math.MaxInt32},
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Enter:
			f.hovered = true
		case pointer.Leave:
			f.hovered = false
		case pointer.Scroll:
			// A wheel notch arrives as one large delta; touchpads send many
			// small ones, which are accumulated into steps.
			y := e.Scroll.Y
			if math.Abs(float64(y)) < 10 {
				f.scroll += y
				if math.Abs(float64(f.scroll)) < 30 {
					break
				}
				y, f.scroll = f.scroll, 0
			}
			if y < 0 {
				s.step(1)
			} else {
				s.step(-1)
			}
		case pointer.Press:
			if e.Buttons.Contain(pointer.ButtonPrimary) {
				f.pressed, f.dragged, f.lastX, f.scrub = true, false, e.Position.X, 0
			}
		case pointer.Drag:
			if !f.pressed {
				break
			}
			f.scrub += e.Position.X - f.lastX
			f.lastX = e.Position.X
			per := float32(gtx.Dp(6))
			for ; f.scrub >= per; f.scrub -= per {
				s.step(1)
				f.dragged = true
			}
			for ; f.scrub <= -per; f.scrub += per {
				s.step(-1)
				f.dragged = true
			}
		case pointer.Release:
			if f.pressed && !f.dragged {
				f.startEditing(s.value)
			}
			f.pressed = false
		case pointer.Cancel:
			f.pressed = false
		}
	}
}

func (f *numField) startEditing(v float64) {
	t := strconv.FormatFloat(v, 'f', -1, 64)
	f.editing, f.focused = true, false
	f.ed.SingleLine, f.ed.Submit = true, true
	f.ed.SetText(t)
	f.ed.SetCaret(len(t), 0)
}

// edit handles the field's editor, applying the value on enter and
// cancelling on escape or when focus moves elsewhere.
func (f *numField) edit(gtx C, s numFieldSpec) {
	if !f.focused {
		gtx.Execute(key.FocusCmd{Tag: &f.ed})
		f.focused = true
	} else if !gtx.Focused(&f.ed) {
		f.editing = false
		return
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &f.ed, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			f.editing = false
			return
		}
	}
	for {
		ev, ok := f.ed.Update(gtx)
		if !ok {
			break
		}
		if e, ok := ev.(widget.SubmitEvent); ok {
			f.editing = false
			txt := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(e.Text), s.unit))
			if v, err := strconv.ParseFloat(txt, 64); err == nil && v > 0 {
				s.set(v)
			}
			return
		}
	}
}

func (u *UI) layoutNumField(gtx C, f *numField, s numFieldSpec) D {
	f.events(gtx, s)
	if f.editing {
		f.edit(gtx, s)
	}
	if f.hovered && s.tip != "" {
		u.tip = s.tip
	}

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Left: 10, Right: 10, Top: 7, Bottom: 7}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return u.text(gtx, s.label+" ", dimCol) }),
			layout.Rigid(func(gtx C) D {
				if !f.editing {
					return u.text(gtx, strconv.FormatFloat(s.value, 'f', -1, 64)+s.unit, textCol)
				}
				gtx.Constraints.Min.X = gtx.Dp(40)
				ed := material.Editor(u.th, &f.ed, "")
				ed.TextSize = u.th.TextSize
				return ed.Layout(gtx)
			}),
		)
	})
	call := m.Stop()

	r := image.Rectangle{Max: dims.Size}
	bg := barBg
	switch {
	case f.editing:
		bg = btnActive
	case f.hovered || f.pressed:
		bg = btnHover
	}
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(r, gtx.Dp(4)).Op(gtx.Ops))
	area := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, f)
	if !f.editing {
		pointer.CursorColResize.Add(gtx.Ops)
	}
	call.Add(gtx.Ops)
	area.Pop()
	return dims
}

// editingNumber reports whether a number field is taking keyboard input.
func (u *UI) editingNumber() bool {
	return u.sizeField.editing || u.zoomField.editing
}
