package main

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	d := NewDocument()
	d.Add(&Edit{Kind: KindText, Points: []Point{{1, 2}}, Color: "#ff0000", Size: 3, Text: "hi", Selected: true})
	d.Add(&Edit{Kind: KindRect, Points: []Point{{0, 0}, {5, 5}}, Filled: true})
	d.Dirty = true
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	if d.Dirty {
		t.Error("document still dirty after save")
	}
	got, err := LoadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Edits) != 2 || got.Edits[0].Text != "hi" || !got.Edits[1].Filled {
		t.Fatalf("loaded edits = %+v", got.Edits)
	}
	if got.Edits[0].Selected {
		t.Error("selection was persisted")
	}
	if id := got.NextID(); id != 3 {
		t.Errorf("NextID after load = %d, want 3", id)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.json")
	d, err := LoadDocument(path)
	if err != nil || d.Path != path || len(d.Edits) != 0 {
		t.Fatalf("LoadDocument(missing) = %+v, %v", d, err)
	}
}
