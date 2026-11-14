package main

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// prompt is a one-line input bar shown above the status line. A prompt
// with a nil submit is a yes/no confirmation.
type prompt struct {
	label   string
	submit  func(u *UI, text string)
	confirm func(u *UI)
	ed      widget.Editor
	focus   bool
	yes, no widget.Clickable

	// choices are clickable suggestions listed above the input.
	choices []string
	picks   []widget.Clickable
}

func (u *UI) ask(label, initial string, submit func(*UI, string), choices ...string) {
	p := &prompt{label: label, submit: submit, focus: true, choices: choices, picks: make([]widget.Clickable, len(choices))}
	p.ed.SingleLine, p.ed.Submit = true, true
	p.ed.SetText(initial)
	p.ed.SetCaret(len(initial), len(initial))
	u.prompt = p
}

func (u *UI) confirm(label string, yes func(*UI)) {
	u.prompt = &prompt{label: label, confirm: yes}
}

// modal reports whether keyboard input belongs to a prompt or text edit.
func (u *UI) modal() bool { return u.prompt != nil || u.typing.active }

func (u *UI) handlePromptKeys(gtx C) {
	p := u.prompt
	filters := []event.Filter{key.Filter{Name: key.NameEscape}}
	if p.confirm != nil {
		filters = append(filters, key.Filter{Name: "Y"}, key.Filter{Name: "N"}, key.Filter{Name: key.NameReturn})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		switch e.Name {
		case key.NameEscape, "N":
			u.prompt = nil
			return
		case "Y", key.NameReturn:
			u.prompt = nil
			p.confirm(u)
			return
		}
	}
}

func (u *UI) layoutPrompt(gtx C) D {
	p := u.prompt
	if p == nil {
		return D{}
	}
	if p.confirm != nil {
		if p.yes.Clicked(gtx) {
			u.prompt = nil
			p.confirm(u)
		}
		if p.no.Clicked(gtx) {
			u.prompt = nil
		}
	} else {
		if p.focus {
			gtx.Execute(key.FocusCmd{Tag: &p.ed})
			p.focus = false
		}
		for {
			ev, ok := p.ed.Update(gtx)
			if !ok {
				break
			}
			if s, ok := ev.(widget.SubmitEvent); ok {
				u.prompt = nil
				p.submit(u, s.Text)
				break
			}
		}
		for i := range p.picks {
			if p.picks[i].Clicked(gtx) {
				u.prompt = nil
				p.submit(u, p.choices[i])
			}
		}
	}
	if len(p.choices) == 0 {
		return u.layoutPromptBar(gtx, p)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			items := make([]layout.FlexChild, len(p.choices))
			for i, c := range p.choices {
				items[i] = layout.Rigid(func(gtx C) D { return u.button(gtx, &p.picks[i], c, false, "") })
			}
			return layout.Inset{Left: 8, Bottom: 4}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
			})
		}),
		layout.Rigid(func(gtx C) D { return u.layoutPromptBar(gtx, p) }),
	)
}

func (u *UI) layoutPromptBar(gtx C, p *prompt) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			paint.FillShape(gtx.Ops, btnHover, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return D{Size: gtx.Constraints.Min}
		},
		func(gtx C) D {
			return layout.Inset{Left: 8, Right: 8, Top: 2, Bottom: 2}.Layout(gtx, func(gtx C) D {
				items := []layout.FlexChild{
					layout.Rigid(func(gtx C) D { return u.label(gtx, p.label, textCol) }),
				}
				if p.confirm != nil {
					items = append(items,
						layout.Rigid(func(gtx C) D { return u.button(gtx, &p.yes, "yes (y)", false, "") }),
						layout.Rigid(func(gtx C) D { return u.button(gtx, &p.no, "no (n)", false, "") }),
					)
				} else {
					items = append(items, layout.Flexed(1, func(gtx C) D {
						return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx C) D {
							ed := material.Editor(u.th, &p.ed, "")
							ed.TextSize = 13
							return ed.Layout(gtx)
						})
					}))
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, items...)
			})
		},
	)
}

// textEdit is the inline editor of the text tool.
type textEdit struct {
	active bool
	focus  bool
	pos    Point
	ed     widget.Editor
}

func (u *UI) startText(p Point) {
	u.typing = textEdit{active: true, focus: true, pos: u.snapPoint(p)}
	u.typing.ed.SingleLine, u.typing.ed.Submit = true, true
}

// commitText places the edited text, if any, and closes the editor.
func (u *UI) commitText() {
	if !u.typing.active {
		return
	}
	u.typing.active = false
	s := u.typing.ed.Text()
	if s == "" {
		return
	}
	d := u.doc()
	d.BeginChange()
	d.Add(&Edit{Kind: KindText, Points: []Point{u.typing.pos}, Color: u.color, Size: u.size, Text: s})
}

func (u *UI) handleTextKeys(gtx C) {
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			u.typing.active = false
			return
		}
	}
}

// layoutTextEdit draws the text tool's editor over the canvas at its world
// position, sized to match the placed text.
func (u *UI) layoutTextEdit(gtx C) {
	t := &u.typing
	if !t.active {
		return
	}
	if t.focus {
		gtx.Execute(key.FocusCmd{Tag: &t.ed})
		t.focus = false
	}
	for {
		ev, ok := t.ed.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			u.commitText()
			return
		}
	}
	defer op.Offset(u.toScreen(t.pos).Round()).Push(gtx.Ops).Pop()
	gtx.Constraints.Min = image.Pt(gtx.Dp(8), 0)
	size := unit.Sp(textSize(u.size) * u.doc().Zoom)

	m := op.Record(gtx.Ops)
	paint.ColorOp{Color: parseColor(u.color)}.Add(gtx.Ops)
	textMat := m.Stop()
	m = op.Record(gtx.Ops)
	paint.ColorOp{Color: guideCol}.Add(gtx.Ops)
	selMat := m.Stop()

	m = op.Record(gtx.Ops)
	dims := t.ed.Layout(gtx, u.th.Shaper, font.Font{}, size, textMat, selMat)
	call := m.Stop()
	r := image.Rectangle{Max: dims.Size}.Inset(-2)
	paint.FillShape(gtx.Ops, guideCol, clip.Stroke{Path: clip.Rect(r).Path(), Width: 1}.Op())
	call.Add(gtx.Ops)
}

func (u *UI) save() {
	if p := u.doc().Path; p != "" {
		u.saveTo(p)
		return
	}
	u.saveAs()
}

func (u *UI) saveAs() {
	initial := u.doc().Path
	if initial == "" {
		initial = "untitled.json"
	}
	u.ask("save as", initial, func(u *UI, path string) { u.saveTo(expandPath(path)) })
}

// saveTo saves the active document to path, or exports it when path ends
// in .png or .svg.
func (u *UI) saveTo(path string) {
	if path == "" {
		return
	}
	d := u.doc()
	var err error
	switch {
	case hasExt(path, ".png"):
		err = ExportPNG(d.Edits, path)
	case hasExt(path, ".svg"):
		err = ExportSVG(d.Edits, path)
	default:
		err = d.Save(path)
	}
	if err != nil {
		u.status = fmt.Sprintf("saving %s failed: %v", path, err)
		return
	}
	if d.Path == path {
		clearAutosave(d)
		u.remember(path)
	}
	u.status = "saved " + path
}

func (u *UI) export() {
	base := strings.TrimSuffix(u.doc().Path, filepath.Ext(u.doc().Path))
	if base == "" {
		base = "untitled"
	}
	u.ask("export (.png or .svg)", base+".png", func(u *UI, path string) { u.saveTo(expandPath(path)) })
}

func (u *UI) open() {
	u.ask("open", "", func(u *UI, path string) { u.openPath(expandPath(path)) }, u.recent...)
}

// openPath opens path in a new tab, or replaces the active tab if it is
// an untouched empty document.
func (u *UI) openPath(path string) {
	if path == "" {
		return
	}
	for i, d := range u.tabs {
		if d.Path == path {
			u.selectTab(i)
			return
		}
	}
	d, err := LoadDocument(path)
	if err != nil {
		u.status = fmt.Sprintf("open failed: %v", err)
		return
	}
	u.addTab(d)
	u.remember(path)
	u.status = "opened " + path
}

// addTab shows d in a new tab, or in place of the active tab if that is an
// untouched empty document.
func (u *UI) addTab(d *Document) {
	if cur := u.doc(); cur.Path == "" && !cur.Dirty && len(cur.Edits) == 0 {
		u.tabs[u.active] = d
		return
	}
	u.tabs = append(u.tabs, d)
	u.active = len(u.tabs) - 1
}

func (u *UI) pickColor() {
	u.ask("color", u.color, func(u *UI, s string) {
		c := s
		if len(c) > 0 && c[0] != '#' {
			c = "#" + c
		}
		if _, ok := parseHex(c); !ok {
			u.status = "invalid color " + s + " (want #rrggbb)"
			return
		}
		u.color = c
	})
}

// requestCloseTab closes tab i, asking first if it has unsaved changes.
func (u *UI) requestCloseTab(i int) {
	if !u.tabs[i].Dirty || len(u.tabs[i].Edits) == 0 {
		u.closeTab(i)
		return
	}
	d := u.tabs[i]
	u.confirm(fmt.Sprintf("discard unsaved changes to %s?", d.Title()), func(u *UI) {
		clearAutosave(d)
		for j, t := range u.tabs {
			if t == d {
				u.closeTab(j)
			}
		}
	})
}

// requestQuit quits, asking first if any tab has unsaved changes.
func (u *UI) requestQuit() {
	n := 0
	for _, d := range u.tabs {
		if d.Dirty && len(d.Edits) > 0 {
			n++
		}
	}
	if n == 0 {
		u.quit()
		return
	}
	u.confirm(fmt.Sprintf("discard unsaved changes in %s and quit?", plural(n, "tab")), func(u *UI) {
		for _, d := range u.tabs {
			clearAutosave(d)
			d.Dirty = false
		}
		u.quit()
	})
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
