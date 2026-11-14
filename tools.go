package main

import (
	"fmt"
	"math"
)

// Tool is the active drawing tool.
type Tool int

const (
	ToolBrush Tool = iota
	ToolLine
	ToolArrow
	ToolRect
	ToolCircle
	ToolEraser
	ToolSelect
	ToolMove
	ToolPicker
	numTools
)

var toolNames = [numTools]string{"brush", "line", "arrow", "rect", "circle", "eraser", "select", "move", "picker"}

func (t Tool) String() string { return toolNames[t] }

var shapeKinds = map[Tool]Kind{ToolLine: KindLine, ToolArrow: KindArrow, ToolRect: KindRect, ToolCircle: KindCircle}

const (
	sizeMin, sizeMax = 1.0, 200.0
	sizeStep         = 1.4
	zoomMin, zoomMax = 0.01, 100.0
	zoomStep         = 1.25
	snapStep         = 16.0
	pasteOffset      = 12.0

	// hitTolerance is the select/move/picker click tolerance, in screen dp.
	hitTolerance = 6.0
	// minStrokeStep is the minimum distance between brush points, in screen dp.
	minStrokeStep = 1.5
	// marqueeThreshold is how far a select drag must go, in screen dp,
	// before it counts as a marquee rather than a click.
	marqueeThreshold = 4.0
)

// drag is the state of an in-progress primary-button drag on the canvas.
type drag struct {
	active  bool
	edit    *Edit   // brush stroke or shape being drawn
	targets []*Edit // edits being moved
	start   Point
	last    Point
	changed bool // an undo point has been recorded for this drag
}

func (u *UI) doc() *Document { return u.tabs[u.active] }

func (u *UI) setTool(t Tool) {
	if u.drag.active {
		return
	}
	u.tool = t
	u.hover = nil
}

func (u *UI) setSize(s float64) {
	u.size = math.Round(max(sizeMin, min(sizeMax, s))*10) / 10
	u.status = fmt.Sprintf("size %g", u.size)
}

func (u *UI) snapPoint(p Point) Point {
	if !u.snap {
		return p
	}
	return Point{math.Round(p.X/snapStep) * snapStep, math.Round(p.Y/snapStep) * snapStep}
}

// worldTol converts a screen distance in dp to world units.
func (u *UI) worldTol(dp float64) float64 { return dp / u.doc().Zoom }

func (u *UI) eraserRadius() float64 { return max(u.size/2, u.worldTol(4)) }

func (u *UI) toolDown(p Point) {
	d := u.doc()
	u.drag = drag{active: true, start: p, last: p}
	switch u.tool {
	case ToolBrush:
		d.BeginChange()
		u.drag.edit = &Edit{Kind: KindStroke, Points: []Point{p}, Color: u.color, Size: u.size}
		d.Add(u.drag.edit)
	case ToolLine, ToolArrow, ToolRect, ToolCircle:
		p = u.snapPoint(p)
		d.BeginChange()
		u.drag.edit = &Edit{Kind: shapeKinds[u.tool], Points: []Point{p, p}, Color: u.color, Size: u.size, Filled: u.filled}
		d.Add(u.drag.edit)
	case ToolEraser:
		u.eraseAt(p)
	case ToolMove:
		u.drag.targets = d.Selected()
		if len(u.drag.targets) == 0 {
			if e := u.editAt(p, u.worldTol(hitTolerance)); e != nil {
				u.drag.targets = []*Edit{e}
			}
		}
	case ToolPicker:
		if e := u.editAt(p, u.worldTol(hitTolerance)); e != nil {
			u.color = e.Color
			u.status = "picked " + e.Color
		}
	}
}

func (u *UI) toolDrag(p Point, constrain bool) {
	d := u.doc()
	switch u.tool {
	case ToolBrush:
		pts := u.drag.edit.Points
		if distance(pts[len(pts)-1], p) >= u.worldTol(minStrokeStep) {
			u.drag.edit.Points = append(pts, p)
		}
	case ToolLine, ToolArrow, ToolRect, ToolCircle:
		e := u.drag.edit
		if constrain {
			p = constrainPoint(e.Kind, e.Points[0], p)
		}
		e.Points[1] = u.snapPoint(p)
	case ToolEraser:
		u.eraseAt(p)
	case ToolMove:
		if len(u.drag.targets) == 0 {
			break
		}
		if !u.drag.changed {
			d.BeginChange()
			u.drag.changed = true
		}
		delta := p.Sub(u.drag.last)
		for _, e := range u.drag.targets {
			e.Translate(delta)
		}
		d.Touch()
	case ToolSelect:
		if distance(u.drag.start, p) >= u.worldTol(marqueeThreshold) {
			u.selectRect(u.drag.start, p)
		}
	}
	u.drag.last = p
}

func (u *UI) toolUp(p Point, constrain bool) {
	if !u.drag.active {
		return
	}
	u.toolDrag(p, constrain)
	if u.tool == ToolSelect && distance(u.drag.start, p) < u.worldTol(marqueeThreshold) {
		if e := u.editAt(p, u.worldTol(hitTolerance)); e != nil {
			e.Selected = !e.Selected
		}
		u.doc().Touch()
	}
	u.drag = drag{}
}

// eraseAt removes every edit touching the eraser at p.
func (u *UI) eraseAt(p Point) {
	d := u.doc()
	r := u.eraserRadius()
	kept := make([]*Edit, 0, len(d.Edits))
	for _, e := range d.Edits {
		if e.Distance(p) > r {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(d.Edits) {
		return
	}
	if !u.drag.changed {
		d.BeginChange()
		u.drag.changed = true
	}
	d.Edits = kept
	u.hover = nil
}

// selectRect selects exactly the edits with a point inside the rect a-b.
func (u *UI) selectRect(a, b Point) {
	lo, hi := rectCorners([]Point{a, b})
	for _, e := range u.doc().Edits {
		e.Selected = false
		for _, p := range e.Points {
			if p.X >= lo.X && p.X <= hi.X && p.Y >= lo.Y && p.Y <= hi.Y {
				e.Selected = true
				break
			}
		}
	}
	u.doc().Touch()
}

// editAt returns the topmost edit within tol of p, or nil.
func (u *UI) editAt(p Point, tol float64) *Edit {
	edits := u.doc().Edits
	for i := len(edits) - 1; i >= 0; i-- {
		if edits[i].Distance(p) <= tol {
			return edits[i]
		}
	}
	return nil
}

// updateHover tracks the edit the eraser or move tool would act on.
func (u *UI) updateHover(p Point) {
	u.hover = nil
	switch u.tool {
	case ToolEraser:
		u.hover = u.editAt(p, u.eraserRadius())
	case ToolMove, ToolPicker:
		u.hover = u.editAt(p, u.worldTol(hitTolerance))
	}
}

func (u *UI) selectAll() {
	d := u.doc()
	for _, e := range d.Edits {
		e.Selected = true
	}
	d.Touch()
}

func (u *UI) deselect() {
	d := u.doc()
	for _, e := range d.Edits {
		e.Selected = false
	}
	d.Touch()
}

func (u *UI) deleteSelected() {
	d := u.doc()
	if len(d.Selected()) == 0 {
		return
	}
	d.BeginChange()
	kept := d.Edits[:0]
	for _, e := range d.Edits {
		if !e.Selected {
			kept = append(kept, e)
		}
	}
	d.Edits = kept
	u.hover = nil
}

func (u *UI) copySelected() {
	sel := u.doc().Selected()
	if len(sel) == 0 {
		return
	}
	u.clipboard = u.clipboard[:0]
	for _, e := range sel {
		u.clipboard = append(u.clipboard, e.Clone())
	}
	u.status = fmt.Sprintf("copied %d", len(sel))
}

func (u *UI) cutSelected() {
	u.copySelected()
	u.deleteSelected()
}

// paste inserts the clipboard slightly offset, selected and ready to move.
func (u *UI) paste() {
	if len(u.clipboard) == 0 {
		return
	}
	d := u.doc()
	d.BeginChange()
	for _, e := range d.Edits {
		e.Selected = false
	}
	for _, src := range u.clipboard {
		src.Translate(Point{pasteOffset, pasteOffset})
		e := src.Clone()
		e.Selected = true
		d.Add(e)
	}
	u.tool = ToolMove
	u.status = fmt.Sprintf("pasted %d", len(u.clipboard))
}

func (u *UI) clearCanvas() {
	d := u.doc()
	if len(d.Edits) == 0 {
		return
	}
	d.BeginChange()
	d.Edits = nil
	u.hover = nil
	u.status = "cleared (undo to restore)"
}

func (u *UI) newTab() {
	u.tabs = append(u.tabs, NewDocument())
	u.active = len(u.tabs) - 1
}

// closeTab closes tab i, keeping at least one (empty) tab open.
func (u *UI) closeTab(i int) {
	u.tabs = append(u.tabs[:i], u.tabs[i+1:]...)
	if len(u.tabs) == 0 {
		u.tabs = []*Document{NewDocument()}
	}
	u.active = min(u.active, len(u.tabs)-1)
	u.drag = drag{}
	u.hover = nil
}

func (u *UI) selectTab(i int) {
	if u.drag.active {
		return
	}
	u.active = (i + len(u.tabs)) % len(u.tabs)
	u.hover = nil
}
