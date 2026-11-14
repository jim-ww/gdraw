package main

import "path/filepath"

// Document is one canvas: its edits plus view and undo state.
type Document struct {
	Path   string
	Dirty  bool
	Edits  []*Edit
	Offset Point // world point at the canvas' top-left corner
	Zoom   float64

	// Version increments on every change that affects rendering.
	Version int

	nextID     int
	history    history
	fills      map[*Edit]*fillEntry
	autosaveID string // names the recovery file, see autosave.go
}

// NewDocument returns an empty, untitled document.
func NewDocument() *Document {
	return &Document{Zoom: 1}
}

// Title is the tab label: base filename or "untitled", plus a dirty marker.
func (d *Document) Title() string {
	name := "untitled"
	if d.Path != "" {
		name = filepath.Base(d.Path)
	}
	if d.Dirty {
		name += " •"
	}
	return name
}

// NextID returns a fresh, document-unique edit ID.
func (d *Document) NextID() int {
	d.nextID++
	return d.nextID
}

// Touch invalidates render caches after a mutation.
func (d *Document) Touch() { d.Version++ }

// Add appends e with a fresh ID.
func (d *Document) Add(e *Edit) {
	e.ID = d.NextID()
	d.Edits = append(d.Edits, e)
	d.Touch()
}

// Selected returns the selected edits.
func (d *Document) Selected() []*Edit {
	var sel []*Edit
	for _, e := range d.Edits {
		if e.Selected {
			sel = append(sel, e)
		}
	}
	return sel
}

// BeginChange records the current state as an undo point. Call it once
// before mutating Edits for a discrete action.
func (d *Document) BeginChange() {
	d.history.push(d.snapshot())
	d.Dirty = true
	d.Touch()
}

func (d *Document) Undo() { d.restore(d.history.undo) }
func (d *Document) Redo() { d.restore(d.history.redo) }

func (d *Document) restore(step func([]*Edit) ([]*Edit, bool)) {
	if s, ok := step(d.snapshot()); ok {
		d.Edits = s
		d.Dirty = true
		d.Touch()
	}
}

func (d *Document) snapshot() []*Edit {
	s := make([]*Edit, len(d.Edits))
	for i, e := range d.Edits {
		s[i] = e.Clone()
	}
	return s
}

// history is a snapshot-based undo/redo stack.
type history struct {
	past, future [][]*Edit
}

func (h *history) push(s []*Edit) {
	h.past = append(h.past, s)
	h.future = nil
}

func (h *history) undo(cur []*Edit) ([]*Edit, bool) {
	return move(&h.past, &h.future, cur)
}

func (h *history) redo(cur []*Edit) ([]*Edit, bool) {
	return move(&h.future, &h.past, cur)
}

// move pops the top of from, pushes cur onto to, and returns the popped state.
func move(from, to *[][]*Edit, cur []*Edit) ([]*Edit, bool) {
	n := len(*from)
	if n == 0 {
		return nil, false
	}
	s := (*from)[n-1]
	*from = (*from)[:n-1]
	*to = append(*to, cur)
	return s, true
}
