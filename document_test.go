package main

import "testing"

func TestUndoRedo(t *testing.T) {
	d := NewDocument()
	d.BeginChange()
	d.Add(&Edit{Kind: KindLine, Points: []Point{{0, 0}, {10, 0}}})
	d.BeginChange()
	d.own(d.Edits[0]).Translate(Point{5, 5})

	d.Undo()
	if got := d.Edits[0].Points[0]; got != (Point{0, 0}) {
		t.Fatalf("after undo: point = %v, want origin", got)
	}
	d.Undo()
	if len(d.Edits) != 0 {
		t.Fatalf("after second undo: %d edits, want 0", len(d.Edits))
	}
	d.Undo() // no-op on empty history
	d.Redo()
	d.Redo()
	if got := d.Edits[0].Points[0]; got != (Point{5, 5}) {
		t.Fatalf("after redo: point = %v, want (5,5)", got)
	}
}

func TestDistance(t *testing.T) {
	tests := []struct {
		name string
		e    Edit
		p    Point
		want float64
	}{
		{"on line", Edit{Kind: KindLine, Points: []Point{{0, 0}, {10, 0}}}, Point{5, 0}, 0},
		{"off line minus width", Edit{Kind: KindLine, Size: 2, Points: []Point{{0, 0}, {10, 0}}}, Point{5, 4}, 3},
		{"inside hollow rect", Edit{Kind: KindRect, Points: []Point{{0, 0}, {10, 10}}}, Point{5, 5}, 5},
		{"inside filled rect", Edit{Kind: KindRect, Filled: true, Points: []Point{{0, 0}, {10, 10}}}, Point{5, 5}, 0},
		{"circle outline", Edit{Kind: KindCircle, Points: []Point{{0, 0}, {10, 10}}}, Point{10, 5}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.Distance(tt.p); got != tt.want {
				t.Errorf("Distance = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstrainPoint(t *testing.T) {
	if got := constrainPoint(KindRect, Point{0, 0}, Point{10, -4}); got != (Point{10, -10}) {
		t.Errorf("rect: got %v", got)
	}
	got := constrainPoint(KindLine, Point{0, 0}, Point{10, 1})
	if got.Y > 1e-9 || got.Y < -1e-9 {
		t.Errorf("line: got %v, want horizontal", got)
	}
}
