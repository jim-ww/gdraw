package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestNumField(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	u := newUI(Config{})
	var f numField
	value := 10.0
	spec := func() numFieldSpec {
		return numFieldSpec{
			label: "size", value: value,
			step: func(n int) { value += float64(n) },
			set:  func(v float64) { value = v },
		}
	}
	var r input.Router
	var ops op.Ops
	frame := func(events ...event.Event) {
		r.Queue(events...)
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Now: time.Now(), Source: r.Source(),
			Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(400, 100)}}
		u.layoutNumField(gtx, &f, spec())
		r.Frame(&ops)
	}
	at := f32.Pt(5, 5)
	mouse := func(k pointer.Kind, pos f32.Point, b pointer.Buttons) pointer.Event {
		return pointer.Event{Kind: k, Source: pointer.Mouse, Position: pos, Buttons: b}
	}

	frame()
	frame(mouse(pointer.Move, at, 0), pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: at, Scroll: f32.Pt(0, -150)})
	frame()
	if value != 11 {
		t.Fatalf("after wheel up: %v, want 11", value)
	}

	frame(mouse(pointer.Press, at, pointer.ButtonPrimary), mouse(pointer.Move, at.Add(f32.Pt(13, 0)), pointer.ButtonPrimary),
		mouse(pointer.Release, at.Add(f32.Pt(13, 0)), 0))
	frame()
	if value != 13 || f.editing {
		t.Fatalf("after drag: value %v, editing %v; want 13, false", value, f.editing)
	}

	frame(mouse(pointer.Press, at, pointer.ButtonPrimary), mouse(pointer.Release, at, 0))
	frame()
	if !f.editing {
		t.Fatal("click did not start editing")
	}
	frame()
	frame(key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "42"}, key.Event{Name: key.NameReturn, State: key.Press})
	frame()
	if value != 42 || f.editing {
		t.Fatalf("after typing: value %v, editing %v; want 42, false", value, f.editing)
	}
}
