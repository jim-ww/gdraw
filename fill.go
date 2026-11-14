package main

import (
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// fillEntry caches the computed area of one fill edit, and its image ops
// per color.
type fillEntry struct {
	seed   Point
	window [2]Point // world area the result depends on
	geom   uint64   // geomHash of the ink within window
	mask   *fillMask
	imgs   map[string]paint.ImageOp
}

func newFillEntry(edits []*Edit, seed Point, m *fillMask, window [2]Point) *fillEntry {
	return &fillEntry{seed: seed, window: window, geom: geomHash(edits, window), mask: m}
}

// updateFills recomputes the areas of fills whose surroundings changed
// and drops entries of removed edits.
func (d *Document) updateFills() {
	next := make(map[*Edit]*fillEntry)
	for _, e := range d.Edits {
		if e.Kind != KindFill || len(e.Points) == 0 {
			continue
		}
		seed := e.Points[0]
		f := d.fills[e]
		if f == nil || f.seed != seed || f.geom != geomHash(d.Edits, f.window) {
			m, w := fillRegion(d.Edits, seed)
			f = newFillEntry(d.Edits, seed, m, w)
		}
		next[e] = f
	}
	d.fills = next
}

// fillAt returns the topmost fill covering p.
func (d *Document) fillAt(p Point) *Edit {
	for i := len(d.Edits) - 1; i >= 0; i-- {
		e := d.Edits[i]
		if f := d.fills[e]; e.Kind == KindFill && f != nil && f.mask != nil && f.mask.contains(p) {
			return e
		}
	}
	return nil
}

func drawFill(ops *op.Ops, f *fillEntry, hex string) {
	if f == nil || f.mask == nil {
		return
	}
	img, ok := f.imgs[hex]
	if !ok {
		img = paint.NewImageOp(f.mask.colorize(parseColor(hex)))
		if f.imgs == nil {
			f.imgs = make(map[string]paint.ImageOp)
		}
		f.imgs[hex] = img
	}
	s := float32(1 / f.mask.scale)
	at := layout.FPt(f.mask.mask.Rect.Min)
	defer op.Affine(f32.AffineId().Offset(at).Scale(f32.Point{}, f32.Pt(s, s)).Offset(fpt(f.mask.origin))).Push(ops).Pop()
	defer clip.Rect{Max: img.Size()}.Push(ops).Pop()
	img.Add(ops)
	paint.PaintOp{}.Add(ops)
}
