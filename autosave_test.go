package main

import (
	"slices"
	"testing"
)

func TestAutosaveRestore(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	d := NewDocument()
	d.Path = "/tmp/x.json"
	d.BeginChange()
	d.Add(&Edit{Kind: KindLine, Points: []Point{{0, 0}, {1, 1}}})
	u := &UI{tabs: []*Document{d, NewDocument()}}
	u.autosave()

	files := recoveryFiles()
	if len(files) != 1 {
		t.Fatalf("recovery files = %v, want 1 (clean tabs are skipped)", files)
	}
	got, err := loadRecovery(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != d.Path || !got.Dirty || len(got.Edits) != 1 || got.autosaveID != d.autosaveID {
		t.Fatalf("restored %+v", got)
	}
	clearAutosave(got)
	if files := recoveryFiles(); len(files) != 0 {
		t.Fatalf("recovery files after clear = %v", files)
	}
}

func TestAddRecent(t *testing.T) {
	r := addRecent([]string{"a", "b", "c"}, "b")
	if !slices.Equal(r, []string{"b", "a", "c"}) {
		t.Errorf("got %v", r)
	}
	for i := range 20 {
		r = addRecent(r, string(rune('d'+i)))
	}
	if len(r) != maxRecent {
		t.Errorf("len = %d, want %d", len(r), maxRecent)
	}
}
