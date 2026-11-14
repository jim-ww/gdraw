package main

import (
	"fmt"
	"strings"
	"unicode"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
)

// action is a named command that keys can be bound to.
type action struct {
	name string
	help string
	keys []string
	run  func(*UI)
}

func toolAction(t Tool, k string) action {
	return action{"tool_" + t.String(), t.String(), []string{k}, func(u *UI) { u.setTool(t) }}
}

func defaultActions() []action {
	acts := []action{
		toolAction(ToolBrush, "b"),
		toolAction(ToolLine, "l"),
		toolAction(ToolArrow, "a"),
		toolAction(ToolRect, "r"),
		toolAction(ToolCircle, "c"),
		toolAction(ToolText, "t"),
		toolAction(ToolFill, "f"),
		toolAction(ToolEraser, "e"),
		toolAction(ToolSelect, "s"),
		toolAction(ToolMove, "m"),
		toolAction(ToolPicker, "i"),

		{"undo", "undo", []string{"ctrl+z"}, func(u *UI) { u.doc().Undo(); u.hover = nil }},
		{"redo", "redo", []string{"ctrl+shift+z", "ctrl+y"}, func(u *UI) { u.doc().Redo(); u.hover = nil }},
		{"select_all", "select all", []string{"ctrl+a"}, (*UI).selectAll},
		{"deselect", "deselect", []string{"esc"}, (*UI).deselect},
		{"delete", "delete selection", []string{"delete", "backspace"}, (*UI).deleteSelected},
		{"copy", "copy selection", []string{"ctrl+c"}, (*UI).copySelected},
		{"cut", "cut selection", []string{"ctrl+x"}, (*UI).cutSelected},
		{"paste", "paste", []string{"ctrl+v"}, (*UI).paste},
		{"clear", "clear canvas", []string{"ctrl+delete"}, (*UI).clearCanvas},

		{"pick_color", "pick color by hex", []string{"p"}, (*UI).pickColor},
		{"toggle_fill", "toggle filled shapes", []string{"ctrl+f"}, func(u *UI) { u.filled = !u.filled }},
		{"toggle_grid", "toggle grid", []string{"g"}, func(u *UI) { u.grid = !u.grid }},
		{"toggle_snap", "toggle snap to grid", []string{"n"}, func(u *UI) { u.snap = !u.snap }},
		{"size_inc", "size up", []string{"=", "+"}, func(u *UI) { u.setSize(u.size * sizeStep) }},
		{"size_dec", "size down", []string{"-"}, func(u *UI) { u.setSize(u.size / sizeStep) }},

		{"zoom_in", "zoom in", []string{"]", "ctrl+="}, func(u *UI) { u.zoomAtCursor(zoomStep) }},
		{"zoom_out", "zoom out", []string{"[", "ctrl+-"}, func(u *UI) { u.zoomAtCursor(1 / zoomStep) }},
		{"zoom_reset", "reset zoom", []string{"ctrl+0"}, (*UI).resetZoom},
		{"pan_up", "pan up", []string{"up"}, func(u *UI) { u.panBy(0, -panStep) }},
		{"pan_down", "pan down", []string{"down"}, func(u *UI) { u.panBy(0, panStep) }},
		{"pan_left", "pan left", []string{"left"}, func(u *UI) { u.panBy(-panStep, 0) }},
		{"pan_right", "pan right", []string{"right"}, func(u *UI) { u.panBy(panStep, 0) }},

		{"open", "open", []string{"ctrl+o"}, (*UI).open},
		{"save", "save", []string{"ctrl+s"}, (*UI).save},
		{"save_as", "save as", []string{"ctrl+shift+s"}, (*UI).saveAs},
		{"export", "export png/svg", []string{"ctrl+e"}, (*UI).export},
		{"new_tab", "new tab", []string{"ctrl+t", "ctrl+n"}, (*UI).newTab},
		{"close_tab", "close tab", []string{"ctrl+w"}, func(u *UI) { u.requestCloseTab(u.active) }},
		{"next_tab", "next tab", []string{"tab"}, func(u *UI) { u.selectTab(u.active + 1) }},
		{"prev_tab", "previous tab", []string{"shift+tab"}, func(u *UI) { u.selectTab(u.active - 1) }},

		{"help", "show keys", []string{"?", "f1"}, func(u *UI) { u.help = true }},
		{"quit", "quit", []string{"ctrl+q"}, (*UI).requestQuit},
	}
	for i := range 9 {
		acts = append(acts, action{
			fmt.Sprintf("color_%d", i+1), fmt.Sprintf("palette color %d", i+1),
			[]string{fmt.Sprint(i + 1)},
			func(u *UI) {
				if i < len(u.palette) {
					u.color = u.palette[i]
				}
			},
		})
	}
	return acts
}

const panStep = 40.0 // dp per arrow key press

// binding is one parsed key combination, e.g. "ctrl+shift+z".
type binding struct {
	name key.Name
	mods key.Modifiers
}

var keyNames = map[string]key.Name{
	"up": key.NameUpArrow, "down": key.NameDownArrow, "left": key.NameLeftArrow, "right": key.NameRightArrow,
	"esc": key.NameEscape, "enter": key.NameReturn, "tab": key.NameTab, "space": key.NameSpace,
	"delete": key.NameDeleteForward, "backspace": key.NameDeleteBackward,
	"home": key.NameHome, "end": key.NameEnd, "pageup": key.NamePageUp, "pagedown": key.NamePageDown,
	"f1": key.NameF1, "f2": key.NameF2, "f3": key.NameF3, "f4": key.NameF4, "f5": key.NameF5, "f6": key.NameF6,
	"f7": key.NameF7, "f8": key.NameF8, "f9": key.NameF9, "f10": key.NameF10, "f11": key.NameF11, "f12": key.NameF12,
}

var modNames = map[string]key.Modifiers{
	"ctrl": key.ModCtrl, "shift": key.ModShift, "alt": key.ModAlt, "super": key.ModSuper,
}

// parseBinding parses a key combination such as "ctrl+shift+z" or "+".
func parseBinding(s string) (binding, error) {
	var b binding
	parts := strings.Split(strings.ToLower(s), "+")
	k := parts[len(parts)-1]
	if k == "" && strings.HasSuffix(s, "+") { // the "+" key itself
		k, parts = "+", parts[:len(parts)-1]
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	} else {
		parts = parts[:len(parts)-1]
	}
	for _, m := range parts {
		mod, ok := modNames[m]
		if !ok {
			return b, fmt.Errorf("unknown modifier %q in %q", m, s)
		}
		b.mods |= mod
	}
	if n, ok := keyNames[k]; ok {
		b.name = n
	} else if r := []rune(k); len(r) == 1 {
		b.name = key.Name(strings.ToUpper(k))
	} else {
		return b, fmt.Errorf("unknown key %q in %q", k, s)
	}
	return b, nil
}

// optional returns the modifiers b tolerates: symbols like "?" or "+" are
// often typed with shift, so shift is ignored for them unless required.
func (b binding) optional() key.Modifiers {
	r := []rune(string(b.name))
	if len(r) == 1 && !unicode.IsLetter(r[0]) && b.mods&key.ModShift == 0 {
		return key.ModShift
	}
	return 0
}

func (b binding) matches(e key.Event) bool {
	return e.Name == b.name && e.Modifiers&^b.optional() == b.mods
}

func (b binding) String() string {
	var sb strings.Builder
	for _, m := range []struct {
		mod  key.Modifiers
		name string
	}{{key.ModCtrl, "ctrl+"}, {key.ModAlt, "alt+"}, {key.ModSuper, "super+"}, {key.ModShift, "shift+"}} {
		if b.mods.Contain(m.mod) {
			sb.WriteString(m.name)
		}
	}
	for s, n := range keyNames {
		if n == b.name {
			sb.WriteString(s)
			return sb.String()
		}
	}
	sb.WriteString(strings.ToLower(string(b.name)))
	return sb.String()
}

// keymap binds actions to keys.
type keymap struct {
	actions  []action
	bindings [][]binding // parallel to actions
	filters  []event.Filter
}

// newKeymap binds acts, letting overrides (action name to keys) replace
// the defaults. Invalid keys are reported and skipped.
func newKeymap(acts []action, overrides map[string][]string) (*keymap, []error) {
	km := &keymap{actions: acts, bindings: make([][]binding, len(acts))}
	var errs []error
	for i, a := range acts {
		keys := a.keys
		if o, ok := overrides[a.name]; ok {
			keys = o
		}
		for _, k := range keys {
			b, err := parseBinding(k)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", a.name, err))
				continue
			}
			km.bindings[i] = append(km.bindings[i], b)
			km.filters = append(km.filters, key.Filter{Name: b.name, Required: b.mods, Optional: b.optional()})
		}
	}
	return km, errs
}

// lookup returns the action bound to e, if any.
func (km *keymap) lookup(e key.Event) (action, bool) {
	for i, bs := range km.bindings {
		for _, b := range bs {
			if b.matches(e) {
				return km.actions[i], true
			}
		}
	}
	return action{}, false
}

// keysFor returns the first key bound to the named action, for tooltips.
func (km *keymap) keysFor(name string) string {
	for i, a := range km.actions {
		if a.name == name && len(km.bindings[i]) > 0 {
			return km.bindings[i][0].String()
		}
	}
	return ""
}

func (u *UI) handleKeys(gtx layout.Context) {
	switch {
	case u.prompt != nil:
		u.handlePromptKeys(gtx)
		return
	case u.typing.active:
		u.handleTextKeys(gtx)
		return
	case u.help:
		u.handleHelpKeys(gtx)
		return
	}
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameSpace, Optional: key.ModShift | key.ModCtrl | key.ModAlt})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok {
			u.spaceDown = e.State == key.Press
		}
	}
	for {
		ev, ok := gtx.Event(u.keymap.filters...)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		if a, ok := u.keymap.lookup(e); ok {
			a.run(u)
		}
	}
}
