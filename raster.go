package main

import (
	"encoding/binary"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// xform maps world coordinates to raster pixels: (p - origin) * scale.
type xform struct {
	origin Point
	scale  float64
}

func (t xform) px(p Point) (float64, float64) {
	return (p.X - t.origin.X) * t.scale, (p.Y - t.origin.Y) * t.scale
}

// coverEdit accumulates the antialiased coverage of e into m. Fills are
// not drawn here; see fillRegion.
func coverEdit(m *image.Alpha, e *Edit, t xform) {
	if len(e.Points) == 0 {
		return
	}
	r := max(0.5, e.Size/2*t.scale)
	switch e.Kind {
	case KindStroke, KindLine:
		coverPolyline(m, e.Points, r, t)
	case KindArrow:
		a, b := e.Points[0], e.Points[len(e.Points)-1]
		l, rr := arrowHead(a, b, e.Size)
		coverPolyline(m, []Point{a, b}, r, t)
		coverPolyline(m, []Point{l, b, rr}, r, t)
	case KindRect:
		lo, hi := rectCorners(e.Points)
		if e.Filled {
			coverRect(m, lo, hi, t)
		}
		coverPolyline(m, rectPath(lo, hi), r, t)
	case KindCircle:
		if e.Filled {
			coverEllipse(m, e.Points, t)
		}
		coverPolyline(m, ellipsePath(e.Points), r, t)
	case KindText:
		coverText(m, e, t)
	}
}

// plot raises the coverage at (x, y) to c.
func plot(m *image.Alpha, x, y int, c float64) {
	if c <= 0 {
		return
	}
	i := m.PixOffset(x, y)
	if v := uint8(min(1, c) * 255); v > m.Pix[i] {
		m.Pix[i] = v
	}
}

// pixelBox clips the pixel box spanning x0..x1, y0..y1 to m.
func pixelBox(m *image.Alpha, x0, y0, x1, y1 float64) image.Rectangle {
	r := image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1))+1, int(math.Ceil(y1))+1)
	return r.Intersect(m.Rect)
}

func coverPolyline(m *image.Alpha, pts []Point, r float64, t xform) {
	if len(pts) == 1 {
		pts = []Point{pts[0], pts[0]}
	}
	for i := range len(pts) - 1 {
		ax, ay := t.px(pts[i])
		bx, by := t.px(pts[i+1])
		a, b := Point{ax, ay}, Point{bx, by}
		box := pixelBox(m, min(ax, bx)-r-1, min(ay, by)-r-1, max(ax, bx)+r+1, max(ay, by)+r+1)
		for y := box.Min.Y; y < box.Max.Y; y++ {
			for x := box.Min.X; x < box.Max.X; x++ {
				d := segmentDistance(a, b, Point{float64(x) + 0.5, float64(y) + 0.5})
				plot(m, x, y, r+0.5-d)
			}
		}
	}
}

func coverRect(m *image.Alpha, lo, hi Point, t xform) {
	x0, y0 := t.px(lo)
	x1, y1 := t.px(hi)
	box := pixelBox(m, x0, y0, x1, y1)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			cx, cy := float64(x)+0.5, float64(y)+0.5
			plot(m, x, y, min(cx-x0, x1-cx, cy-y0, y1-cy)+0.5)
		}
	}
}

func coverEllipse(m *image.Alpha, pts []Point, t xform) {
	lo, hi := rectCorners(pts)
	x0, y0 := t.px(lo)
	x1, y1 := t.px(hi)
	cx, cy := (x0+x1)/2, (y0+y1)/2
	rx, ry := (x1-x0)/2, (y1-y0)/2
	if rx <= 0 || ry <= 0 {
		return
	}
	box := pixelBox(m, x0, y0, x1, y1)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			n := math.Hypot((float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry)
			plot(m, x, y, 0.5-(n-1)*min(rx, ry))
		}
	}
}

var goFont = sync.OnceValue(func() *opentype.Font {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err) // embedded font; cannot fail
	}
	return f
})

func coverText(m *image.Alpha, e *Edit, t xform) {
	face, err := opentype.NewFace(goFont(), &opentype.FaceOptions{Size: textSize(e.Size) * t.scale, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return
	}
	defer face.Close()
	x, y := t.px(e.Points[0])
	d := font.Drawer{Src: image.Opaque, Face: face}
	d.Dot = fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y*64) + face.Metrics().Ascent}
	b, _ := d.BoundString(e.Text)
	r := image.Rect(b.Min.X.Floor(), b.Min.Y.Floor(), b.Max.X.Ceil(), b.Max.Y.Ceil()).Intersect(m.Rect)
	if r.Empty() {
		return
	}
	glyphs := image.NewAlpha(r)
	d.Dst = glyphs
	d.DrawString(e.Text)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			plot(m, x, y, float64(glyphs.AlphaAt(x, y).A)/255)
		}
	}
}

// fillMask is the area a fill edit covers, as a pixel mask in world space.
type fillMask struct {
	xform
	mask *image.Alpha
}

// fillLevels are the raster scales a fill is attempted at, finest first.
// Each covers fillWindow pixels around the seed, so coarser levels reach
// larger enclosed areas.
var fillLevels = [...]float64{4, 1, 1.0 / 4, 1.0 / 16}

const fillWindow = 1024

// fillRegion floods the area around seed enclosed by the ink of edits. It
// returns nil if seed is on ink or the area isn't enclosed. window is the
// world area whose ink the result depends on.
func fillRegion(edits []*Edit, seed Point) (m *fillMask, window [2]Point) {
	ink := image.NewAlpha(image.Rect(0, 0, fillWindow, fillWindow))
	for _, s := range fillLevels {
		half := fillWindow / 2 / s
		t := xform{origin: Point{seed.X - half, seed.Y - half}, scale: s}
		window = [2]Point{t.origin, {seed.X + half, seed.Y + half}}
		clear(ink.Pix)
		for _, e := range edits {
			if e.Kind != KindFill && intersects(e, window) {
				coverEdit(ink, e, t)
			}
		}
		region, box, enclosed := flood(ink, image.Pt(fillWindow/2, fillWindow/2))
		if region == nil {
			return nil, window
		}
		if enclosed {
			return &fillMask{xform: t, mask: dilate(region, box)}, window
		}
	}
	return nil, window
}

func intersects(e *Edit, w [2]Point) bool {
	lo, hi := e.Bounds()
	return hi.X >= w[0].X && hi.Y >= w[0].Y && lo.X <= w[1].X && lo.Y <= w[1].Y
}

// flood returns the 4-connected area of pixels with less than half ink
// coverage reachable from seed, its bounding box, and whether it stays
// clear of the border.
func flood(ink *image.Alpha, seed image.Point) (region []bool, box image.Rectangle, enclosed bool) {
	const solid = 128
	w, h := ink.Rect.Dx(), ink.Rect.Dy()
	start := seed.Y*w + seed.X
	if ink.Pix[start] >= solid {
		return nil, box, false
	}
	region = make([]bool, w*h)
	region[start] = true
	box = image.Rectangle{Min: seed, Max: seed.Add(image.Pt(1, 1))}
	stack := []int32{int32(start)}
	for len(stack) > 0 {
		i := int(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
		x, y := i%w, i/w
		if x == 0 || y == 0 || x == w-1 || y == h-1 {
			return region, box, false
		}
		box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
		box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
		for _, j := range [4]int{i + 1, i - 1, i + w, i - w} {
			if !region[j] && ink.Pix[j] < solid {
				region[j] = true
				stack = append(stack, int32(j))
			}
		}
	}
	return region, box, true
}

// dilate returns the region (fillWindow pixels wide) within box grown by
// one pixel, so a fill tucks under its antialiased border. The result
// covers just box plus that margin.
func dilate(region []bool, box image.Rectangle) *image.Alpha {
	out := image.NewAlpha(box.Inset(-1))
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if !region[y*fillWindow+x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				i := out.PixOffset(x-1, y+dy)
				out.Pix[i], out.Pix[i+1], out.Pix[i+2] = 255, 255, 255
			}
		}
	}
	return out
}

// bounds returns the world rectangle the mask covers.
func (f *fillMask) bounds() (lo, hi Point) {
	r := f.mask.Rect
	lo = Point{f.origin.X + float64(r.Min.X)/f.scale, f.origin.Y + float64(r.Min.Y)/f.scale}
	hi = Point{f.origin.X + float64(r.Max.X)/f.scale, f.origin.Y + float64(r.Max.Y)/f.scale}
	return lo, hi
}

// colorize returns the mask as a premultiplied image in color c, with
// its top-left pixel at the image origin.
func (f *fillMask) colorize(c color.NRGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.mask.Rect.Dx(), f.mask.Rect.Dy()))
	for i, a := range f.mask.Pix {
		if a == 0 {
			continue
		}
		img.Pix[i*4+0] = uint8(uint16(c.R) * uint16(a) / 255)
		img.Pix[i*4+1] = uint8(uint16(c.G) * uint16(a) / 255)
		img.Pix[i*4+2] = uint8(uint16(c.B) * uint16(a) / 255)
		img.Pix[i*4+3] = a
	}
	return img
}

// contains reports whether world point p lies inside the mask.
func (f *fillMask) contains(p Point) bool {
	x, y := f.px(p)
	q := image.Pt(int(math.Floor(x)), int(math.Floor(y)))
	return q.In(f.mask.Rect) && f.mask.Pix[f.mask.PixOffset(q.X, q.Y)] != 0
}

// geomHash fingerprints the geometry of the non-fill edits intersecting
// window, so cached fills are recomputed only when the ink around them may
// have changed.
func geomHash(edits []*Edit, window [2]Point) uint64 {
	h := fnv.New64a()
	var b [8]byte
	f := func(v float64) {
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
		h.Write(b[:])
	}
	for _, e := range edits {
		if e.Kind == KindFill || !intersects(e, window) {
			continue
		}
		h.Write([]byte(e.Kind))
		h.Write([]byte(e.Text))
		f(e.Size)
		if e.Filled {
			f(1)
		}
		for _, p := range e.Points {
			f(p.X)
			f(p.Y)
		}
	}
	return h.Sum64()
}
