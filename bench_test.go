package main

import (
	"image"
	"math"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// benchUI returns a UI showing a document of n scribbled strokes of 60
// points each, plus a frame function that lays out and routes one frame.
func benchUI(b *testing.B, n int) (*UI, func()) {
	b.Setenv("XDG_DATA_HOME", b.TempDir())
	u := newUI(Config{})
	d := u.doc()
	for i := range n {
		e := &Edit{Kind: KindStroke, Color: "#ffffff", Size: 3}
		x0, y0 := float64(i%40)*30, float64(i/40)*30
		for j := range 60 {
			t := float64(j) / 10
			e.Points = append(e.Points, Point{x0 + t*5, y0 + 10*math.Sin(t)})
		}
		d.Add(e)
	}
	var r input.Router
	var ops op.Ops
	return u, func() {
		ops.Reset()
		gtx := layout.Context{
			Ops: &ops, Now: time.Now(), Source: r.Source(),
			Metric:      unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5},
			Constraints: layout.Exact(image.Pt(1800, 1200)),
		}
		u.Layout(gtx)
		r.Frame(&ops)
	}
}

// BenchmarkFrameIdle is a frame where only the cursor moved.
func BenchmarkFrameIdle(b *testing.B) {
	_, frame := benchUI(b, 1000)
	frame()
	for b.Loop() {
		frame()
	}
}

// BenchmarkFrameNewStroke is the frame after a stroke is finished.
func BenchmarkFrameNewStroke(b *testing.B) {
	u, frame := benchUI(b, 1000)
	frame()
	for b.Loop() {
		u.toolDown(Point{10, 10})
		u.toolDrag(Point{50, 30}, false)
		u.toolUp(Point{90, 10}, false)
		frame()
	}
}

// BenchmarkFrameUndo is the frame after undoing a stroke.
func BenchmarkFrameUndo(b *testing.B) {
	u, frame := benchUI(b, 1000)
	frame()
	for b.Loop() {
		u.toolDown(Point{10, 10})
		u.toolUp(Point{90, 10}, false)
		u.doc().Undo()
		frame()
	}
}

// BenchmarkFrameDrawing is a frame while dragging out a long stroke.
func BenchmarkFrameDrawing(b *testing.B) {
	u, frame := benchUI(b, 1000)
	u.toolDown(Point{0, 0})
	for i := range 2000 {
		u.toolDrag(Point{float64(i), float64(i % 50)}, false)
	}
	frame()
	for b.Loop() {
		frame()
	}
}

// BenchmarkHover is the eraser hit test run on every mouse move.
func BenchmarkHover(b *testing.B) {
	u, _ := benchUI(b, 1000)
	u.tool = ToolEraser
	for b.Loop() {
		u.updateHover(Point{600, 300})
	}
}
