package main

import (
	"encoding/xml"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleEdits() []*Edit {
	edits := []*Edit{
		{Kind: KindRect, Size: 3, Color: "#ffffff", Points: []Point{{0, 0}, {120, 80}}},
		{Kind: KindFill, Color: "#0000ff", Points: []Point{{60, 40}}},
		{Kind: KindCircle, Size: 2, Color: "#ff0000", Filled: true, Points: []Point{{150, 0}, {230, 80}}},
		{Kind: KindArrow, Size: 2, Color: "#00ff00", Points: []Point{{0, 120}, {200, 140}}},
		{Kind: KindStroke, Size: 6, Color: "#ffff00", Points: []Point{{10, 170}, {40, 190}, {80, 165}, {120, 200}}},
		{Kind: KindText, Size: 3, Color: "#ff00ff", Text: `a<b & "c"`, Points: []Point{{140, 160}}},
	}
	bakeFills(edits)
	return edits
}

func TestRasterize(t *testing.T) {
	img := Rasterize(sampleEdits(), exportScale)
	lo, _, _ := drawingBounds(sampleEdits())
	at := func(p Point) [3]uint8 {
		c := img.RGBAAt(int((p.X-lo.X)*exportScale), int((p.Y-lo.Y)*exportScale))
		return [3]uint8{c.R, c.G, c.B}
	}
	checks := []struct {
		name string
		p    Point
		want [3]uint8
	}{
		{"fill", Point{60, 40}, [3]uint8{0, 0, 255}},
		{"rect border", Point{0, 40}, [3]uint8{255, 255, 255}},
		{"filled circle", Point{190, 40}, [3]uint8{255, 0, 0}},
		{"background", Point{140, 100}, [3]uint8{canvasBg.R, canvasBg.G, canvasBg.B}},
	}
	for _, c := range checks {
		if got := at(c.p); got != c.want {
			t.Errorf("%s at %v = %v, want %v", c.name, c.p, got, c.want)
		}
	}
	if out := os.Getenv("GDRAW_TEST_PNG"); out != "" {
		f, _ := os.Create(out)
		png.Encode(f, img)
		f.Close()
	}
}

func TestExportSVGIsWellFormed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.svg")
	if err := ExportSVG(sampleEdits(), path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		if _, err := dec.Token(); err != nil {
			if err.Error() != "EOF" {
				t.Fatalf("invalid SVG: %v", err)
			}
			break
		}
	}
	for _, want := range []string{"<image", "<ellipse", "<polyline", "a&lt;b &amp;"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("SVG lacks %q", want)
		}
	}
}
