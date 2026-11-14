package main

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
)

const (
	exportScale  = 2    // pixels per world unit in PNG exports
	exportPad    = 16.0 // world units of margin around the drawing
	exportMaxDim = 16384
)

// drawingBounds returns the world rectangle enclosing all edits plus
// padding, or false if there is nothing to draw.
func drawingBounds(edits []*Edit) (lo, hi Point, ok bool) {
	lo = Point{math.Inf(1), math.Inf(1)}
	hi = Point{math.Inf(-1), math.Inf(-1)}
	for _, e := range edits {
		if e.Kind == KindFill && e.Area == nil {
			continue
		}
		elo, ehi := e.Bounds()
		lo = Point{min(lo.X, elo.X), min(lo.Y, elo.Y)}
		hi = Point{max(hi.X, ehi.X), max(hi.Y, ehi.Y)}
		ok = true
	}
	pad := Point{exportPad, exportPad}
	return lo.Sub(pad), hi.Add(pad), ok
}

// Rasterize renders edits onto an opaque image at scale pixels per world
// unit (lowered if the image would get too large).
func Rasterize(edits []*Edit, scale float64) *image.RGBA {
	lo, hi, ok := drawingBounds(edits)
	if !ok {
		lo, hi = Point{}, Point{exportPad, exportPad}
	}
	scale = min(scale, exportMaxDim/max(hi.X-lo.X, hi.Y-lo.Y))
	t := xform{origin: lo, scale: scale}
	img := image.NewRGBA(image.Rect(0, 0, int(math.Ceil((hi.X-lo.X)*scale)), int(math.Ceil((hi.Y-lo.Y)*scale))))
	draw.Draw(img, img.Rect, image.NewUniform(canvasBg), image.Point{}, draw.Src)

	for _, e := range edits {
		if e.Kind == KindFill && e.Area != nil {
			paintFill(img, e.Area, t, parseColor(e.Color))
		}
	}
	for _, e := range edits {
		if e.Kind == KindFill {
			continue
		}
		elo, ehi := e.Bounds()
		x0, y0 := t.px(elo)
		x1, y1 := t.px(ehi)
		mask := image.NewAlpha(pixelBox(&image.Alpha{Rect: img.Rect}, x0, y0, x1, y1))
		coverEdit(mask, e, t)
		draw.DrawMask(img, mask.Rect, image.NewUniform(parseColor(e.Color)), image.Point{}, mask, mask.Rect.Min, draw.Over)
	}
	return img
}

// paintFill composites fill mask m onto img, sampling it per image pixel.
func paintFill(img *image.RGBA, m *FillArea, t xform, c color.NRGBA) {
	flo, fhi := m.bounds()
	x0, y0 := t.px(flo)
	x1, y1 := t.px(fhi)
	box := image.Rect(int(x0), int(y0), int(math.Ceil(x1)), int(math.Ceil(y1))).Intersect(img.Rect)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			w := Point{t.origin.X + (float64(x)+0.5)/t.scale, t.origin.Y + (float64(y)+0.5)/t.scale}
			if m.contains(w) {
				img.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, 255})
			}
		}
	}
}

// ExportPNG writes edits as a PNG image.
func ExportPNG(edits []*Edit, path string) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, Rasterize(edits, exportScale)); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes())
}

// ExportSVG writes edits as an SVG document. Fills are embedded as images.
func ExportSVG(edits []*Edit, path string) error {
	data, err := marshalSVG(edits)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

func marshalSVG(edits []*Edit) ([]byte, error) {
	lo, hi, ok := drawingBounds(edits)
	if !ok {
		lo, hi = Point{}, Point{exportPad, exportPad}
	}
	var b strings.Builder
	w, h := hi.X-lo.X, hi.Y-lo.Y
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="%s %s %s %s">`+"\n",
		num(w), num(h), num(lo.X), num(lo.Y), num(w), num(h))
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", num(lo.X), num(lo.Y), num(w), num(h), hexOf(canvasBg))

	for _, e := range edits {
		m := e.Area
		if e.Kind != KindFill || m == nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, m.colorize(parseColor(e.Color))); err != nil {
			return nil, err
		}
		flo, fhi := m.bounds()
		fmt.Fprintf(&b, `<image x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none" href="data:image/png;base64,%s"/>`+"\n",
			num(flo.X), num(flo.Y), num(fhi.X-flo.X), num(fhi.Y-flo.Y), base64.StdEncoding.EncodeToString(buf.Bytes()))
	}
	for _, e := range edits {
		if len(e.Points) > 0 && e.Kind != KindFill {
			writeSVGEdit(&b, e)
		}
	}
	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

func writeSVGEdit(b *strings.Builder, e *Edit) {
	col := hexOf(parseColor(e.Color))
	stroke := fmt.Sprintf(`stroke="%s" stroke-width="%s" stroke-linecap="round" stroke-linejoin="round"`, col, num(e.Size))
	fill := "none"
	if e.Filled {
		fill = col
	}
	switch e.Kind {
	case KindStroke, KindLine:
		if isDot(e.Points) {
			p := e.Points[0]
			fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`+"\n", num(p.X), num(p.Y), num(e.Size/2), col)
			return
		}
		fmt.Fprintf(b, `<polyline points="%s" fill="none" %s/>`+"\n", points(e.Points), stroke)
	case KindArrow:
		a, z := e.Points[0], e.Points[len(e.Points)-1]
		l, r := arrowHead(a, z, e.Size)
		fmt.Fprintf(b, `<polyline points="%s" fill="none" %s/>`+"\n", points([]Point{a, z}), stroke)
		fmt.Fprintf(b, `<polyline points="%s" fill="none" %s/>`+"\n", points([]Point{l, z, r}), stroke)
	case KindRect:
		lo, hi := rectCorners(e.Points)
		fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s" %s/>`+"\n",
			num(lo.X), num(lo.Y), num(hi.X-lo.X), num(hi.Y-lo.Y), fill, stroke)
	case KindCircle:
		lo, hi := rectCorners(e.Points)
		fmt.Fprintf(b, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" %s/>`+"\n",
			num((lo.X+hi.X)/2), num((lo.Y+hi.Y)/2), num((hi.X-lo.X)/2), num((hi.Y-lo.Y)/2), fill, stroke)
	case KindText:
		p := e.Points[0]
		fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" font-family="Go, sans-serif" font-size="%s" dominant-baseline="text-before-edge" xml:space="preserve">`,
			num(p.X), num(p.Y), col, num(textSize(e.Size)))
		xml.EscapeText(b, []byte(e.Text))
		b.WriteString("</text>\n")
	}
}

func points(pts []Point) string {
	s := make([]string, len(pts))
	for i, p := range pts {
		s[i] = num(p.X) + "," + num(p.Y)
	}
	return strings.Join(s, " ")
}

// num formats v compactly with at most two decimals.
func num(v float64) string {
	return fmt.Sprint(math.Round(v*100) / 100)
}

func hexOf(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
