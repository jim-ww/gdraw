package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// FillArea is the area a fill edit paints: an alpha mask placed in world
// space. It is computed once, when the fill is made, so later ink drawn
// across it doesn't change it. Masks are never mutated, so clones share
// them.
type FillArea struct {
	Origin Point        // world position of the mask's top-left corner
	Scale  float64      // mask pixels per world unit
	Mask   *image.Alpha // bounds start at (0, 0)
}

func (a *FillArea) px(p Point) (float64, float64) {
	return (p.X - a.Origin.X) * a.Scale, (p.Y - a.Origin.Y) * a.Scale
}

// bounds returns the world rectangle the mask covers.
func (a *FillArea) bounds() (lo, hi Point) {
	r := a.Mask.Rect
	return a.Origin, Point{a.Origin.X + float64(r.Dx())/a.Scale, a.Origin.Y + float64(r.Dy())/a.Scale}
}

// contains reports whether world point p lies inside the area.
func (a *FillArea) contains(p Point) bool {
	x, y := a.px(p)
	q := image.Pt(int(math.Floor(x)), int(math.Floor(y)))
	return q.In(a.Mask.Rect) && a.Mask.Pix[a.Mask.PixOffset(q.X, q.Y)] != 0
}

// colorize returns the mask as a premultiplied image in color c.
func (a *FillArea) colorize(c color.NRGBA) *image.RGBA {
	img := image.NewRGBA(a.Mask.Rect)
	for i, v := range a.Mask.Pix {
		if v == 0 {
			continue
		}
		img.Pix[i*4+0] = uint8(uint16(c.R) * uint16(v) / 255)
		img.Pix[i*4+1] = uint8(uint16(c.G) * uint16(v) / 255)
		img.Pix[i*4+2] = uint8(uint16(c.B) * uint16(v) / 255)
		img.Pix[i*4+3] = v
	}
	return img
}

type fillAreaJSON struct {
	Origin Point   `json:"origin"`
	Scale  float64 `json:"scale"`
	Mask   []byte  `json:"mask"` // grayscale PNG
}

func (a *FillArea) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	gray := &image.Gray{Pix: a.Mask.Pix, Stride: a.Mask.Stride, Rect: a.Mask.Rect}
	if err := png.Encode(&buf, gray); err != nil {
		return nil, err
	}
	return json.Marshal(fillAreaJSON{a.Origin, a.Scale, buf.Bytes()})
}

func (a *FillArea) UnmarshalJSON(data []byte) error {
	var j fillAreaJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	img, err := png.Decode(bytes.NewReader(j.Mask))
	if err != nil {
		return fmt.Errorf("fill mask: %w", err)
	}
	if j.Scale <= 0 {
		return fmt.Errorf("fill mask: invalid scale %v", j.Scale)
	}
	b := img.Bounds()
	m := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	if g, ok := img.(*image.Gray); ok {
		for y := range b.Dy() {
			copy(m.Pix[y*m.Stride:(y+1)*m.Stride], g.Pix[(y)*g.Stride:])
		}
	} else {
		draw.Draw(m, m.Rect, img, b.Min, draw.Src)
	}
	*a = FillArea{Origin: j.Origin, Scale: j.Scale, Mask: m}
	return nil
}

// bakeFills computes the areas of fills that have none, as in files saved
// by bdraw, which stores only the clicked point.
func bakeFills(edits []*Edit) {
	for _, e := range edits {
		if e.Kind == KindFill && e.Area == nil && len(e.Points) > 0 {
			e.Area = fillRegion(edits, e.Points[0])
		}
	}
}

type fillImageKey struct {
	mask  *image.Alpha
	color string
}

// fillImages caches the GPU images of fill masks per color. Entries not
// used since the previous prune are dropped by the next one.
type fillImages struct {
	cur, prev map[fillImageKey]paint.ImageOp
}

func (c *fillImages) prune() {
	c.prev, c.cur = c.cur, make(map[fillImageKey]paint.ImageOp)
}

func (c *fillImages) get(a *FillArea, hex string) paint.ImageOp {
	k := fillImageKey{a.Mask, hex}
	img, ok := c.cur[k]
	if !ok {
		if img, ok = c.prev[k]; !ok {
			img = paint.NewImageOp(a.colorize(parseColor(hex)))
		}
		if c.cur == nil {
			c.cur = make(map[fillImageKey]paint.ImageOp)
		}
		c.cur[k] = img
	}
	return img
}

func (r *renderer) drawFill(ops *op.Ops, a *FillArea, hex string) {
	if a == nil {
		return
	}
	img := r.fills.get(a, hex)
	s := float32(1 / a.Scale)
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(s, s)).Offset(fpt(a.Origin))).Push(ops).Pop()
	defer clip.Rect{Max: img.Size()}.Push(ops).Pop()
	img.Add(ops)
	paint.PaintOp{}.Add(ops)
}
