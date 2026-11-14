package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
			m := fillRegion(tt.edits, tt.seed)
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

func TestFillKeepsAreaWhenSplit(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	u := newUI(Config{})
	d := u.doc()
	d.Add(&Edit{Kind: KindRect, Size: 2, Points: []Point{{0, 0}, {200, 100}}})
	u.tool = ToolFill
	u.toolDown(Point{50, 50})
	u.toolUp(Point{50, 50}, false)

	// A rect crossing the filled one splits its interior.
	u.tool = ToolRect
	u.toolDown(Point{100, -50})
	u.toolUp(Point{150, 150}, false)

	fill := d.fillAt(Point{50, 50})
	if fill == nil {
		t.Fatal("no fill at the clicked point")
	}
	if d.fillAt(Point{180, 50}) != fill {
		t.Error("fill lost the part cut off by the crossing rect")
	}
}

func TestFillAreaJSON(t *testing.T) {
	box := &Edit{Kind: KindRect, Size: 2, Points: []Point{{0, 0}, {100, 60}}}
	fill := &Edit{Kind: KindFill, Points: []Point{{50, 30}}, Area: fillRegion([]*Edit{box}, Point{50, 30})}
	data, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	var got Edit
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Area.Origin != fill.Area.Origin || got.Area.Scale != fill.Area.Scale || !bytes.Equal(got.Area.Mask.Pix, fill.Area.Mask.Pix) {
		t.Error("area changed in JSON round trip")
	}
}

func TestLoadBakesBdrawFills(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.json")
	old := `{"version":1,"edits":[
		{"id":1,"kind":"rect","points":[{"X":0,"Y":0},{"X":100,"Y":60}],"color":"#ffffff","size":2},
		{"id":2,"kind":"fill","points":[{"X":50,"Y":30}],"color":"#ff0000","size":0}]}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.fillAt(Point{10, 10}) == nil {
		t.Error("bdraw fill has no area after load")
	}
}

func BenchmarkFillRegion(b *testing.B) {
	edits := []*Edit{{Kind: KindRect, Size: 2, Points: []Point{{0, 0}, {800, 500}}}}
	for b.Loop() {
		fillRegion(edits, Point{400, 250})
	}
}
