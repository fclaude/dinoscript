package desktop

import (
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

func (e *editor) tabTitle(t *fileTab) string {
	if t.Path != "" {
		return filepath.Base(t.Path)
	}
	if t.Name != "" {
		return t.Name
	}
	return e.text("Untitled.ds")
}

func (e *editor) restoreTabs() {
	session, ok := readWorkspace(e.draftPath)
	if !ok {
		for i, index := range []int{0, 2, 1, numberMagicExample} {
			name, source := e.sample(index)
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
	name := e.text("Untitled.ds")
	for n := 2; slices.ContainsFunc(e.tabs, func(t *fileTab) bool { return e.tabTitle(t) == name }); n++ {
		name = e.text("Untitled %d.ds", n)
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

func (e *editor) sample(index int) (name, source string) {
	lesson := lessons[index]
	if name, ok := lesson.localized[e.language]; ok {
		return name, example(string(e.language) + "/" + name)
	}
	return lesson.file, example(lesson.file)
}

func (e *editor) openExample(index int) {
	e.openSample(e.sample(index))
	e.setStatus("Opened %s", statusMessage(lessons[index].title))
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
	e.outputScroll.SetPosition(0, 0)
	e.outputScroll.MarkForLayoutAndRedraw()
	e.diagnostic = nil
	e.updateJump()
}

func (e *editor) updateTabTitle() {
	title := e.tabTitle(e.document)
	if e.dirty() {
		title += " *"
	}
	e.window.SetTitle(title + " — Dinoscript")
	if text := shortName(title, 42); e.fileTitle.String() != text {
		e.fileTitle.SetTitle(text)
		e.fileTitle.MarkForLayoutAndRedraw()
	}
	e.styleTab(e.document)
}

func shortName(name string, limit int) string {
	r := []rune(name)
	if len(r) <= limit {
		return name
	}
	return string(r[:limit-7]) + "…" + string(r[len(r)-6:])
}
