package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/richardwilkes/unison"
)

type fileTab struct {
	draft
	button           *unison.Button
	undo             *unison.UndoManager
	selection        *unison.FieldState
	scrollX, scrollY float32
}

func (t *fileTab) title() string {
	if t.Path != "" {
		return filepath.Base(t.Path)
	}
	if t.Name != "" {
		return t.Name
	}
	return "Untitled.ds"
}

func (e *editor) restoreTabs() {
	session, ok := readWorkspace(e.draftPath)
	if !ok {
		for i, name := range []string{"hello.ds", "counting.ds", "decisions.ds"} {
			source := example(name)
			session.Tabs = append(session.Tabs, draft{Name: name, Color: i, Text: source, SavedText: source})
		}
	}
	for _, d := range session.Tabs {
		e.addTab(d)
	}
	e.selectTab(e.tabs[session.Active])
}

func (e *editor) addTab(d draft) *fileTab {
	t := &fileTab{draft: d, undo: unison.NewUndoManager(1024*1024, nil)}
	t.Color = max(t.Color, 0) % len(tabColors)
	t.button = e.fileButton(t)
	e.styleTab(t)
	e.tabs = append(e.tabs, t)
	e.tabList.AddChild(t.button)
	e.tabList.MarkForLayoutAndRedraw()
	return t
}

func (e *editor) openTab(d draft) {
	if len(e.tabs) > 0 {
		d.Color = (e.tabs[len(e.tabs)-1].Color + 1) % len(tabColors)
	}
	e.selectTab(e.addTab(d))
}

func (e *editor) newTab() {
	name := "Untitled.ds"
	for n := 2; slices.ContainsFunc(e.tabs, func(t *fileTab) bool { return t.title() == name }); n++ {
		name = fmt.Sprintf("Untitled %d.ds", n)
	}
	e.openTab(draft{Name: name})
}

func (e *editor) closeTab() {
	if e.dirty() && !e.dialogs.discardChanges() {
		return
	}
	i := slices.Index(e.tabs, e.document)
	e.document.button.RemoveFromParent()
	e.tabs = slices.Delete(e.tabs, i, i+1)
	if len(e.tabs) == 0 {
		e.newTab()
	} else {
		e.selectTab(e.tabs[min(i, len(e.tabs)-1)])
	}
	e.tabList.MarkForLayoutAndRedraw()
}

func (e *editor) hasUnsavedTabs() bool {
	return slices.ContainsFunc(e.tabs, func(t *fileTab) bool { return t.Text != t.SavedText })
}

func (e *editor) tabForPath(path string) *fileTab {
	info, _ := os.Stat(path)
	for _, t := range e.tabs {
		if t.Path == "" {
			continue
		}
		if filepath.Clean(t.Path) == filepath.Clean(path) {
			return t
		}
		if other, err := os.Stat(t.Path); err == nil && info != nil && os.SameFile(info, other) {
			return t
		}
	}
	return nil
}

func (e *editor) openExample(name, title string) {
	e.openSample(name, example(name))
	e.setStatus("Opened " + title)
}

func (e *editor) openSample(name, source string) {
	for _, t := range e.tabs {
		if t.Path == "" && t.Name == name {
			e.selectTab(t)
			return
		}
	}
	e.openTab(draft{Name: name, Text: source, SavedText: source})
}

func (e *editor) selectTab(t *fileTab) {
	if e.document == t {
		e.window.SetFocus(e.source)
		return
	}
	old := e.document
	if old != nil {
		old.selection = e.source.GetFieldState()
		old.scrollX, old.scrollY = e.sourceScroll.Position()
	}
	if e.active != nil {
		e.active.cancel()
		e.active = nil
	}
	e.document = t
	if old != nil {
		e.styleTab(old)
	}
	state := t.selection
	if state == nil {
		state = &unison.FieldState{Text: t.Text}
	}
	e.source.ApplyFieldState(state)
	e.sourceScroll.SetPosition(t.scrollX, t.scrollY)
	e.clearOutput()
	e.setInputEnabled(false)
	e.run.SetEnabled(true)
	e.stop.SetEnabled(false)
	e.window.SetFocus(e.source)
	e.updateDocument()
	e.setStatus("Ready.")
	t.button.ScrollIntoView()
}

func (e *editor) clearOutput() {
	e.outputText.Reset()
	e.output.ApplyFieldState(&unison.FieldState{})
	e.diagnostic = nil
	e.updateJump()
	e.output.MarkForLayoutAndRedraw()
}

func (e *editor) updateTabTitle() {
	title := e.document.title()
	if e.dirty() {
		title += " *"
	}
	e.window.SetTitle(title + " — Dinoscript")
	e.fileTitle.SetTitle(shortName(title, 42))
	e.fileTitle.MarkForLayoutAndRedraw()
	e.styleTab(e.document)
}

func shortName(name string, limit int) string {
	r := []rune(name)
	if len(r) <= limit {
		return name
	}
	return string(r[:limit-7]) + "…" + string(r[len(r)-6:])
}
