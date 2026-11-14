package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

var (
	canvasBg  = color.NRGBA{0x12, 0x12, 0x12, 0xff}
	gridColor = color.NRGBA{0x3a, 0x3a, 0x3a, 0xff}
	guideCol  = color.NRGBA{0xcc, 0xcc, 0xcc, 0x90}
)

// scale is canvas pixels per world unit.
func (u *UI) scale() float64 { return u.doc().Zoom * float64(u.pxPerDp) }

func (u *UI) toWorld(p f32.Point) Point {
	s, o := u.scale(), u.doc().Offset
	return Point{o.X + float64(p.X)/s, o.Y + float64(p.Y)/s}
}

func (u *UI) toScreen(p Point) f32.Point {
	s, o := u.scale(), u.doc().Offset
	return f32.Pt(float32((p.X-o.X)*s), float32((p.Y-o.Y)*s))
}

// zoomAt zooms by factor, keeping the world point under screen point p fixed.
func (u *UI) zoomAt(factor float64, p f32.Point) {
	d := u.doc()
	before := u.toWorld(p)
	d.Zoom = max(zoomMin, min(zoomMax, d.Zoom*factor))
	s := u.scale()
	d.Offset = Point{before.X - float64(p.X)/s, before.Y - float64(p.Y)/s}
}

// zoomAtCursor zooms around the mouse when it's over the canvas, else the
// canvas center.
func (u *UI) zoomAtCursor(factor float64) {
	p := layout.FPt(u.canvasSize).Mul(0.5)
	if u.cursorIn {
		p = u.cursor
	}
	u.zoomAt(factor, p)
}

func (u *UI) resetZoom() {
	u.zoomAtCursor(1 / u.doc().Zoom)
}

// panBy moves the view by (dx, dy) screen dp.
func (u *UI) panBy(dx, dy float64) {
	d := u.doc()
	d.Offset = d.Offset.Add(Point{dx / d.Zoom, dy / d.Zoom})
}

func (u *UI) layoutCanvas(gtx layout.Context) layout.Dimensions {
	u.canvasSize = gtx.Constraints.Max
	u.canvasEvents(gtx)

	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: canvasBg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	if u.grid {
		u.drawGrid(gtx.Ops, size)
	}

	d := u.doc()
	s := float32(u.scale())
	view := op.Affine(f32.AffineId().Offset(fpt(d.Offset).Mul(-1)).Scale(f32.Point{}, f32.Pt(s, s))).Push(gtx.Ops)
	u.scene.draw(gtx.Ops, &u.render, d, u.drag.edit)
	if e := u.drag.edit; e != nil {
		u.render.drawEdit(gtx.Ops, d, e, e.Color)
	}
	if u.hover != nil && !u.drag.active {
		u.render.drawEdit(gtx.Ops, d, u.hover, hoverColor)
	}
	view.Pop()

	if u.drag.active && u.tool == ToolSelect {
		a, b := u.toScreen(u.drag.start), u.toScreen(u.drag.last)
		r := clip.Rect(image.Rectangle{Min: a.Round(), Max: b.Round()}.Canon())
		paint.FillShape(gtx.Ops, guideCol, clip.Stroke{Path: r.Path(), Width: 1}.Op())
	}
	if u.cursorIn && (u.tool == ToolBrush || u.tool == ToolEraser) {
		r := float32(u.size/2) * s
		if u.tool == ToolEraser {
			r = float32(u.eraserRadius()) * s
		}
		r = max(r, 2)
		h := f32.Pt(r, r)
		paint.FillShape(gtx.Ops, guideCol, clip.Stroke{Path: ellipseSpec(gtx.Ops, u.cursor.Sub(h), u.cursor.Add(h)), Width: 1}.Op())
	}

	event.Op(gtx.Ops, u)
	cursor := pointer.CursorCrosshair
	switch {
	case u.pan.active:
		cursor = pointer.CursorGrabbing
	case u.spaceDown:
		cursor = pointer.CursorGrab
	}
	cursor.Add(gtx.Ops)
	u.layoutTextEdit(gtx)
	return layout.Dimensions{Size: size}
}

func (u *UI) canvasEvents(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  u,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Scroll | pointer.Leave,
			ScrollY: pointer.ScrollRange{Min: math.MinInt32, Max: math.MaxInt32},
		})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		u.cursor, u.cursorIn = e.Position, e.Kind != pointer.Leave
		p := u.toWorld(e.Position)
		constrain := e.Modifiers.Contain(key.ModCtrl)
		switch e.Kind {
		case pointer.Scroll:
			f := max(-1, min(1, float64(-e.Scroll.Y)/40))
			u.zoomAt(math.Pow(zoomStep, f), e.Position)
		case pointer.Press:
			if u.drag.active || u.pan.active {
				break
			}
			if e.Buttons.Contain(pointer.ButtonTertiary) || (u.spaceDown && e.Buttons.Contain(pointer.ButtonPrimary)) {
				u.pan.active, u.pan.last = true, e.Position
				break
			}
			if e.Buttons.Contain(pointer.ButtonPrimary) {
				u.toolDown(p)
			}
		case pointer.Drag:
			if u.pan.active {
				delta := e.Position.Sub(u.pan.last)
				u.pan.last = e.Position
				u.panBy(float64(-delta.X)/float64(u.pxPerDp), float64(-delta.Y)/float64(u.pxPerDp))
			} else if u.drag.active {
				u.toolDrag(p, constrain)
			}
		case pointer.Release:
			if u.pan.active {
				u.pan.active = false
			} else if !e.Buttons.Contain(pointer.ButtonPrimary) {
				u.toolUp(p, constrain)
			}
		case pointer.Cancel:
			u.pan.active = false
			u.toolUp(u.drag.last, false)
		case pointer.Move:
			u.updateHover(p)
		}
	}
}

// drawGrid draws a dot grid whose spacing adapts to the zoom level.
func (u *UI) drawGrid(ops *op.Ops, size image.Point) {
	s := u.scale()
	step := snapStep
	minPx := 14 * float64(u.pxPerDp)
	for step*s < minPx {
		step *= 2
	}
	for step*s >= 4*minPx {
		step /= 2
	}
	o := u.doc().Offset
	r := float32(max(1, float64(u.pxPerDp)))
	var p clip.Path
	p.Begin(ops)
	for wx := math.Ceil(o.X/step) * step; (wx-o.X)*s < float64(size.X); wx += step {
		for wy := math.Ceil(o.Y/step) * step; (wy-o.Y)*s < float64(size.Y); wy += step {
			c := f32.Pt(float32((wx-o.X)*s), float32((wy-o.Y)*s))
			p.MoveTo(c)
			p.LineTo(c.Add(f32.Pt(r, 0)))
			p.LineTo(c.Add(f32.Pt(r, r)))
			p.LineTo(c.Add(f32.Pt(0, r)))
			p.Close()
		}
	}
	paint.FillShape(ops, gridColor, clip.Outline{Path: p.End()}.Op())
}
