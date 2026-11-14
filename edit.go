package main

import "math"

// Kind identifies the shape of an Edit.
type Kind string

const (
	KindStroke Kind = "stroke"
	KindRect   Kind = "rect"
	KindCircle Kind = "circle"
	KindLine   Kind = "line"
	KindArrow  Kind = "arrow"
	KindText   Kind = "text"
	KindFill   Kind = "fill"
)

// Point is a location in world space. One world unit is one dp at 100% zoom.
type Point struct {
	X, Y float64
}

func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

func distance(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// Edit is a single undoable unit of drawing: one stroke, shape, text or fill.
// The JSON shape matches bdraw project files.
type Edit struct {
	ID     int     `json:"id"`
	Kind   Kind    `json:"kind"`
	Points []Point `json:"points"`
	Color  string  `json:"color"`
	Size   float64 `json:"size"`
	Text   string  `json:"text,omitempty"`
	Filled bool    `json:"filled,omitempty"`

	Area *FillArea `json:"area,omitempty"` // fills only

	Selected bool `json:"-"`
}

// Clone returns a deep copy of e.
func (e *Edit) Clone() *Edit {
	c := *e
	c.Points = append([]Point(nil), e.Points...)
	if e.Area != nil {
		a := *e.Area
		c.Area = &a
	}
	return &c
}

// Translate moves every point of e by d.
func (e *Edit) Translate(d Point) {
	for i := range e.Points {
		e.Points[i] = e.Points[i].Add(d)
	}
	if e.Area != nil {
		e.Area.Origin = e.Area.Origin.Add(d)
	}
}

// Bounds returns e's bounding box, including stroke width.
func (e *Edit) Bounds() (lo, hi Point) {
	if e.Area != nil {
		return e.Area.bounds()
	}
	lo = Point{math.Inf(1), math.Inf(1)}
	hi = Point{math.Inf(-1), math.Inf(-1)}
	for _, p := range e.Points {
		lo = Point{min(lo.X, p.X), min(lo.Y, p.Y)}
		hi = Point{max(hi.X, p.X), max(hi.Y, p.Y)}
	}
	pad := e.Size/2 + 1
	switch e.Kind {
	case KindArrow:
		pad += arrowHeadLen(e.Size)
	case KindText:
		if len(e.Points) > 0 {
			hi = Point{lo.X + textWidth(e), lo.Y + textSize(e.Size)*1.3}
		}
	}
	return lo.Sub(Point{pad, pad}), hi.Add(Point{pad, pad})
}

// Distance returns the shortest distance from p to e's geometry. It drives
// the eraser, selection and eyedropper hit tests.
func (e *Edit) Distance(p Point) float64 {
	if len(e.Points) == 0 {
		return math.Inf(1)
	}
	switch e.Kind {
	case KindStroke, KindLine, KindArrow:
		return max(0, polylineDistance(e.Points, p)-e.Size/2)
	case KindRect:
		lo, hi := rectCorners(e.Points)
		if e.Filled && p.X >= lo.X && p.X <= hi.X && p.Y >= lo.Y && p.Y <= hi.Y {
			return 0
		}
		return max(0, polylineDistance(rectPath(lo, hi), p)-e.Size/2)
	case KindCircle:
		return max(0, ellipseDistance(e.Points, p, e.Filled)-e.Size/2)
	case KindText:
		lo, hi := e.Bounds()
		if p.X >= lo.X && p.X <= hi.X && p.Y >= lo.Y && p.Y <= hi.Y {
			return 0
		}
		return distance(e.Points[0], p)
	case KindFill:
		if e.Area != nil && e.Area.contains(p) {
			return 0
		}
		return math.Inf(1)
	default:
		return distance(e.Points[0], p)
	}
}

func segmentDistance(a, b, p Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return distance(a, p)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / lenSq
	t = max(0, min(1, t))
	return distance(Point{a.X + t*dx, a.Y + t*dy}, p)
}

func polylineDistance(pts []Point, p Point) float64 {
	if len(pts) == 1 {
		return distance(pts[0], p)
	}
	d := math.Inf(1)
	for i := range len(pts) - 1 {
		d = min(d, segmentDistance(pts[i], pts[i+1], p))
	}
	return d
}

// rectCorners normalizes two opposite corners into min/max corners.
func rectCorners(pts []Point) (lo, hi Point) {
	a, b := pts[0], pts[len(pts)-1]
	return Point{min(a.X, b.X), min(a.Y, b.Y)}, Point{max(a.X, b.X), max(a.Y, b.Y)}
}

func rectPath(lo, hi Point) []Point {
	return []Point{lo, {hi.X, lo.Y}, hi, {lo.X, hi.Y}, lo}
}

// ellipseDistance approximates the distance from p to the ellipse whose
// bounding box is spanned by pts. Exact for circles.
func ellipseDistance(pts []Point, p Point, filled bool) float64 {
	lo, hi := rectCorners(pts)
	c := Point{(lo.X + hi.X) / 2, (lo.Y + hi.Y) / 2}
	rx, ry := (hi.X-lo.X)/2, (hi.Y-lo.Y)/2
	if rx == 0 || ry == 0 {
		return segmentDistance(lo, hi, p)
	}
	n := math.Hypot((p.X-c.X)/rx, (p.Y-c.Y)/ry)
	if filled && n <= 1 {
		return 0
	}
	return math.Abs(n-1) * min(rx, ry)
}

// ellipsePath approximates the ellipse spanned by pts as a closed polyline.
func ellipsePath(pts []Point) []Point {
	lo, hi := rectCorners(pts)
	c := Point{(lo.X + hi.X) / 2, (lo.Y + hi.Y) / 2}
	rx, ry := (hi.X-lo.X)/2, (hi.Y-lo.Y)/2
	n := int(max(16, min(720, (rx+ry)*math.Pi/2)))
	out := make([]Point, n+1)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = Point{c.X + rx*math.Cos(a), c.Y + ry*math.Sin(a)}
	}
	return out
}

func arrowHeadLen(size float64) float64 { return max(10, size*4) }

// arrowHead returns the two barb end points of an arrow from a to b.
func arrowHead(a, b Point, size float64) (Point, Point) {
	const spread = math.Pi / 7
	l := arrowHeadLen(size)
	ang := math.Atan2(b.Y-a.Y, b.X-a.X) + math.Pi
	return Point{b.X + l*math.Cos(ang-spread), b.Y + l*math.Sin(ang-spread)},
		Point{b.X + l*math.Cos(ang+spread), b.Y + l*math.Sin(ang+spread)}
}

// textSize is the font size, in world units, of a text edit of brush size s.
func textSize(s float64) float64 { return 12 + 4*s }

// textWidth estimates the rendered width of a text edit.
func textWidth(e *Edit) float64 {
	return float64(len([]rune(e.Text))) * textSize(e.Size) * 0.6
}

// constrainPoint snaps lines and arrows to 45° steps and makes rects and
// ellipses square, relative to start.
func constrainPoint(kind Kind, start, pt Point) Point {
	d := pt.Sub(start)
	switch kind {
	case KindLine, KindArrow:
		l := math.Hypot(d.X, d.Y)
		if l == 0 {
			return pt
		}
		a := math.Round(math.Atan2(d.Y, d.X)/(math.Pi/4)) * (math.Pi / 4)
		return Point{start.X + l*math.Cos(a), start.Y + l*math.Sin(a)}
	case KindRect, KindCircle:
		s := max(math.Abs(d.X), math.Abs(d.Y))
		return Point{start.X + math.Copysign(s, d.X), start.Y + math.Copysign(s, d.Y)}
	}
	return pt
}
