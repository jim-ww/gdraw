package main

import (
	"testing"

	"gioui.org/io/key"
)

func TestParseBinding(t *testing.T) {
	tests := []struct {
		in   string
		want binding
	}{
		{"b", binding{"B", 0}},
		{"ctrl+shift+z", binding{"Z", key.ModCtrl | key.ModShift}},
		{"+", binding{"+", 0}},
		{"ctrl++", binding{"+", key.ModCtrl}},
		{"esc", binding{key.NameEscape, 0}},
		{"shift+tab", binding{key.NameTab, key.ModShift}},
	}
	for _, tt := range tests {
		got, err := parseBinding(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseBinding(%q) = %v, %v; want %v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"hyper+b", "nosuchkey"} {
		if _, err := parseBinding(bad); err == nil {
			t.Errorf("parseBinding(%q) succeeded, want error", bad)
		}
	}
}

func TestBindingMatches(t *testing.T) {
	tests := []struct {
		bind string
		ev   key.Event
		want bool
	}{
		{"z", key.Event{Name: "Z"}, true},
		{"ctrl+z", key.Event{Name: "Z", Modifiers: key.ModCtrl}, true},
		{"ctrl+z", key.Event{Name: "Z", Modifiers: key.ModCtrl | key.ModShift}, false},
		{"b", key.Event{Name: "B", Modifiers: key.ModShift}, false},
		{"?", key.Event{Name: "?", Modifiers: key.ModShift}, true},
		{"+", key.Event{Name: "+"}, true},
	}
	for _, tt := range tests {
		b, _ := parseBinding(tt.bind)
		if got := b.matches(tt.ev); got != tt.want {
			t.Errorf("%q matches %v = %v, want %v", tt.bind, tt.ev, got, tt.want)
		}
	}
}

func TestDefaultKeymapValid(t *testing.T) {
	if _, errs := newKeymap(defaultActions(), nil); len(errs) > 0 {
		t.Fatal(errs)
	}
}
