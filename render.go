package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

const (
	selectColor = "#ffaa00"
	hoverColor  = "#00ffaa"
)

func fpt(p Point) f32.Point { return f32.Pt(float32(p.X), float32(p.Y)) }

// scene draws documents from per-edit recorded ops. Each edit's ops
// survive until the edit changes, so frames only replay them and Gio can
// reuse its tessellated paths.
type scene struct {
	doc *Document
	ver int
}

// editOps is the recorded drawing of one edit in one color.
type editOps struct {
	ops   op.Ops
	call  op.CallOp
	rev   uint64
	color string
}

// draw adds d's edits visible in view (a world rectangle) in world
// coordinates, except skip, which the caller draws live while it's being
// dragged.
func (s *scene) draw(ops *op.Ops, r *renderer, d *Document, skip *Edit, view [2]Point) {
	if s.doc != d || s.ver != d.Version {
		s.doc, s.ver = d, d.Version
		s.prune(d)
		r.fills.prune()
	}
	// Fills go first, underneath the ink that encloses them.
	for _, fills := range []bool{true, false} {
		for _, e := range d.Edits {
			if e == skip || (e.Kind == KindFill) != fills || !intersects(e, view) {
				continue
			}
			c := e.Color
			if e.Selected {
				c = selectColor
			}
			r.recorded(d, e, c).Add(ops)
		}
	}
}

// prune drops the cached ops of edits no longer in d.
func (s *scene) prune(d *Document) {
	if len(d.ops) <= len(d.Edits) {
		return
	}
	live := make(map[int]bool, len(d.Edits))
	for _, e := range d.Edits {
		live[e.ID] = true
	}
	for id := range d.ops {
		if !live[id] {
			delete(d.ops, id)
		}
	}
}

// recorded returns e's ops in color c, recording them if e changed. They
// are keyed by ID, so the clones that undo restores reuse them too.
func (r *renderer) recorded(d *Document, e *Edit, c string) op.CallOp {
	if d.ops == nil {
		d.ops = make(map[int]*editOps)
	}
	eo := d.ops[e.ID]
	if eo == nil {
		eo = new(editOps)
		d.ops[e.ID] = eo
	}
	if eo.rev != e.rev || eo.color != c || e.rev == 0 {
		eo.rev, eo.color = e.rev, c
		eo.ops.Reset()
		m := op.Record(&eo.ops)
		r.drawEdit(&eo.ops, d, e, c)
		eo.call = m.Stop()
	}
	return eo.call
}

// renderer draws edits as Gio ops in world coordinates.
type renderer struct {
	shaper *text.Shaper
	fills  fillImages
}

func (r *renderer) drawEdit(ops *op.Ops, d *Document, e *Edit, hex string) {
	if len(e.Points) == 0 {
		return
	}
	col := parseColor(hex)
	w := float32(max(e.Size, 0.1))
	switch e.Kind {
	case KindStroke, KindLine:
		strokePolyline(ops, e.Points, w, col)
	case KindArrow:
		a, b := e.Points[0], e.Points[len(e.Points)-1]
		l, rr := arrowHead(a, b, e.Size)
		strokePolyline(ops, []Point{a, b}, w, col)
		strokePolyline(ops, []Point{l, b, rr}, w, col)
	case KindRect:
		lo, hi := rectCorners(e.Points)
		if e.Filled {
			paint.FillShape(ops, col, clip.Outline{Path: polyPath(ops, rectPath(lo, hi))}.Op())
		}
		strokePolyline(ops, rectPath(lo, hi), w, col)
	case KindCircle:
		lo, hi := rectCorners(e.Points)
		if e.Filled {
			paint.FillShape(ops, col, clip.Outline{Path: ellipseSpec(ops, fpt(lo), fpt(hi))}.Op())
		}
		paint.FillShape(ops, col, clip.Stroke{Path: ellipseSpec(ops, fpt(lo), fpt(hi)), Width: w}.Op())
	case KindText:
		r.drawText(ops, e, col)
	case KindFill:
		r.drawFill(ops, e.Area, hex)
	}
}

// strokePolyline strokes pts with round caps and joins; a degenerate
// polyline (a single click) becomes a dot.
func strokePolyline(ops *op.Ops, pts []Point, w float32, col color.NRGBA) {
	if isDot(pts) {
		p, h := fpt(pts[0]), f32.Pt(w/2, w/2)
		paint.FillShape(ops, col, clip.Outline{Path: ellipseSpec(ops, p.Sub(h), p.Add(h))}.Op())
		return
	}
	paint.FillShape(ops, col, clip.Stroke{Path: polyPath(ops, pts), Width: w}.Op())
}

func isDot(pts []Point) bool {
	for _, p := range pts[1:] {
		if p != pts[0] {
			return false
		}
	}
	return true
}

func polyPath(ops *op.Ops, pts []Point) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(fpt(pts[0]))
	for _, q := range pts[1:] {
		p.LineTo(fpt(q))
	}
	return p.End()
}

// ellipseSpec returns the ellipse inscribed in the box lo-hi as four cubic
// Bézier arcs.
func ellipseSpec(ops *op.Ops, lo, hi f32.Point) clip.PathSpec {
	const k = 0.5522848 // control point distance for a quarter circle
	c := lo.Add(hi).Mul(0.5)
	rx, ry := (hi.X-lo.X)/2, (hi.Y-lo.Y)/2
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(c.X+rx, c.Y))
	p.CubeTo(f32.Pt(c.X+rx, c.Y+k*ry), f32.Pt(c.X+k*rx, c.Y+ry), f32.Pt(c.X, c.Y+ry))
	p.CubeTo(f32.Pt(c.X-k*rx, c.Y+ry), f32.Pt(c.X-rx, c.Y+k*ry), f32.Pt(c.X-rx, c.Y))
	p.CubeTo(f32.Pt(c.X-rx, c.Y-k*ry), f32.Pt(c.X-k*rx, c.Y-ry), f32.Pt(c.X, c.Y-ry))
	p.CubeTo(f32.Pt(c.X+k*rx, c.Y-ry), f32.Pt(c.X+rx, c.Y-k*ry), f32.Pt(c.X+rx, c.Y))
	p.Close()
	return p.End()
}

func (r *renderer) drawText(ops *op.Ops, e *Edit, col color.NRGBA) {
	defer op.Affine(f32.AffineId().Offset(fpt(e.Points[0]))).Push(ops).Pop()
	gtx := layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(math.MaxInt32/2, math.MaxInt32/2)},
	}
	m := op.Record(ops)
	paint.ColorOp{Color: col}.Add(ops)
	widget.Label{MaxLines: 1}.Layout(gtx, r.shaper, font.Font{}, unit.Sp(textSize(e.Size)), e.Text, m.Stop())
}

// parseColor parses "#rrggbb", falling back to white.
func parseColor(s string) color.NRGBA {
	c, ok := parseHex(s)
	if !ok {
		return color.NRGBA{255, 255, 255, 255}
	}
	return c
}

func parseHex(s string) (color.NRGBA, bool) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, false
	}
	var v [3]uint8
	for i := range v {
		hi, ok1 := hexDigit(s[1+2*i])
		lo, ok2 := hexDigit(s[2+2*i])
		if !ok1 || !ok2 {
			return color.NRGBA{}, false
		}
		v[i] = hi<<4 | lo
	}
	return color.NRGBA{v[0], v[1], v[2], 255}, true
}

func hexDigit(c byte) (uint8, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
