package main

import "testing"

func TestFillRegion(t *testing.T) {
	box := &Edit{Kind: KindRect, Size: 2, Points: []Point{{0, 0}, {100, 60}}}
	ring := &Edit{Kind: KindCircle, Size: 2, Points: []Point{{-2000, -2000}, {2000, 2000}}}
	open := &Edit{Kind: KindLine, Size: 2, Points: []Point{{0, 0}, {100, 0}}}

	tests := []struct {
		name   string
		edits  []*Edit
		seed   Point
		inside []Point
		out    []Point
	}{
		{"inside rect", []*Edit{box}, Point{50, 30}, []Point{{5, 5}, {95, 55}}, []Point{{-5, 30}, {50, 70}}},
		{"large ellipse", []*Edit{ring}, Point{0, 0}, []Point{{1500, 0}}, []Point{{2100, 0}}},
		{"unenclosed", []*Edit{open}, Point{50, 30}, nil, nil},
		{"on ink", []*Edit{box}, Point{0, 30}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := fillRegion(tt.edits, tt.seed)
			if (m != nil) != (tt.inside != nil) {
				t.Fatalf("fillRegion returned %v, want region: %v", m != nil, tt.inside != nil)
			}
			for _, p := range tt.inside {
				if !m.contains(p) {
					t.Errorf("%v not inside fill", p)
				}
			}
			for _, p := range tt.out {
				if m.contains(p) {
					t.Errorf("%v inside fill", p)
				}
			}
		})
	}
}

func TestGeomHashIgnoresSelectionAndFills(t *testing.T) {
	w := [2]Point{{-10, -10}, {10, 10}}
	a := &Edit{Kind: KindLine, Points: []Point{{0, 0}, {1, 1}}}
	far := &Edit{Kind: KindLine, Points: []Point{{100, 100}, {101, 101}}}
	h := geomHash([]*Edit{a}, w)
	a.Selected = true
	if geomHash([]*Edit{a, far, {Kind: KindFill, Points: []Point{{3, 3}}}}, w) != h {
		t.Error("hash changed on selection, fill or ink outside the window")
	}
	a.Translate(Point{1, 0})
	if geomHash([]*Edit{a}, w) == h {
		t.Error("hash unchanged after move")
	}
}

func BenchmarkFillRegion(b *testing.B) {
	edits := []*Edit{{Kind: KindRect, Size: 2, Points: []Point{{0, 0}, {800, 500}}}}
	for b.Loop() {
		fillRegion(edits, Point{400, 250})
	}
}
