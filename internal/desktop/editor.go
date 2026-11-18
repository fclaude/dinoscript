package desktop

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
)

//go:embed examples/*.ds
var examples embed.FS

var lessons = []struct {
	title string
	file  string
}{
	{"Hello, dinosaur!", "hello.ds"},
	{"Make a decision", "decisions.ds"},
	{"Count with a loop", "counting.ds"},
	{"Fibonacci numbers", "fibonacci.ds"},
	{"Is it prime?", "prime.ds"},
}

type editor struct {
	window *unison.Window
	source *unison.Field
	output *unison.Field
	input  *unison.Field
	status *unison.Label
	run    *unison.Button
	stop   *unison.Button
	jump   *unison.Button
	learn  *unison.Button
	undo   *unison.UndoManager
	reader *tutorialReader

	dialogs     fileDialogs
	draftPath   string
	draftQueued bool
	document    draft
	closed      bool

	active           *execution
	outputText       strings.Builder
	diagnostic       *dino.Diagnostic
	diagnosticSource string
}

// Open opens the offline editor from the desktop toolkit's UI thread.
func Open() error {
	var path string
	if dir, err := os.UserConfigDir(); err == nil {
		path = filepath.Join(dir, "Dinoscript", "draft.json")
	}
	_, err := newEditor(path, systemDialogs{})
	return err
}

func newEditor(draftPath string, dialogs fileDialogs) (*editor, error) {
	window, err := unison.NewWindow("Dinoscript")
	if err != nil {
		return nil, err
	}
	e := &editor{
		window: window, dialogs: dialogs, draftPath: draftPath,
		source: unison.NewMultiLineField(), output: unison.NewMultiLineField(),
		input: unison.NewField(), status: unison.NewLabel(),
		undo: unison.NewUndoManager(1024*1024, nil),
	}
	e.buildWindow()
	first := example(lessons[0].file)
	initial := draft{Text: first, SavedText: first}
	if recovered, ok := readDraft(draftPath); ok {
		initial = recovered
	}
	e.load(initial)
	e.setStatus("Ready.")
	window.KeyDownCallback = e.onKey
	window.AllowCloseCallback = func() bool {
		if err := e.writeDraft(); err != nil && e.dirty() {
			return e.dialogs.discardChanges()
		}
		return true
	}
	window.WillCloseCallback = func() {
		e.closed = true
		if e.reader != nil {
			e.reader.window.Dispose()
		}
		if e.active != nil {
			e.active.cancel()
		}
	}
	window.PackWithDefaultInitialLocation()
	window.ToFront()
	window.SetFocus(e.source)
	return e, nil
}

func example(name string) string {
	source, err := examples.ReadFile("examples/" + name)
	if err != nil {
		panic(err) // Only names in the embedded lesson list are used.
	}
	return string(source)
}

func (e *editor) buildWindow() {
	content := e.window.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 10})
	content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(12)))

	exampleMenu := unison.NewPopupMenu[string]()
	exampleMenu.Accessibility.Name = "Examples"
	exampleMenu.AddDisabledItem("Examples")
	exampleMenu.Select("Examples")
	for _, lesson := range lessons {
		exampleMenu.AddItem(lesson.title)
	}
	exampleMenu.ChoiceMadeCallback = func(_ *unison.PopupMenu[string], _ int, title string) {
		for _, lesson := range lessons {
			if lesson.title == title {
				e.openExample(lesson.file, title)
				return
			}
		}
	}
	e.run = button("Run (F5)", e.startRun)
	e.stop = button("Stop", e.stopRun)
	e.stop.SetEnabled(false)
	e.jump = button("Go to error", e.jumpToError)
	e.jump.SetEnabled(false)

	toolbar := unison.NewPanel()
	toolbar.SetLayout(&unison.FlexLayout{Columns: 8, HSpacing: 7, VAlign: align.Middle})
	toolbar.AddChild(e.run)
	toolbar.AddChild(e.stop)
	toolbar.AddChild(exampleMenu)
	toolbar.AddChild(button("Open…", e.open))
	toolbar.AddChild(button("Save", e.save))
	e.learn = button("Learn", e.openTutorial)
	toolbar.AddChild(e.learn)
	toolbar.AddChild(button("Smaller text", func() { e.zoom(-1) }))
	toolbar.AddChild(button("Larger text", func() { e.zoom(1) }))
	content.AddChild(toolbar)

	e.source.SetWrap(false)
	e.source.NoSelectAllOnFocus = true
	e.source.ModifiedCallback = e.edited
	e.source.KeyDownCallback = e.codeKey
	e.output.Watermark = "Run your code to see what it prints."
	e.output.NoSelectAllOnFocus = true
	e.output.RuneTypedCallback = func(rune) bool { return false }
	e.output.KeyDownCallback = func(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
		switch key {
		case unison.KeyBackspace, unison.KeyDelete, unison.KeyReturn, unison.KeyNumPadEnter:
			return true
		case unison.KeyX, unison.KeyV:
			if mods.OSMenuCommandDown() {
				return true
			}
		}
		return e.output.DefaultKeyDown(key, mods, repeat)
	}
	e.output.ModifiedCallback = func(before, _ *unison.FieldState) { e.output.ApplyFieldState(before) }
	for _, id := range []int{unison.CutItemID, unison.PasteItemID, unison.DeleteItemID} {
		e.output.InstallCmdHandlers(id, func(any) bool { return false }, nil)
	}
	e.output.Accessibility.Callback = func(node *accessibility.Node) {
		node.ReadOnly = true
		node.Actions = node.Actions.Without(accessibility.SetValue, accessibility.ReplaceText)
	}
	e.input.Watermark = "Program input"
	e.input.Accessibility.Name = "Program input"
	e.input.SetEnabled(false)
	e.input.KeyDownCallback = func(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
		if key == unison.KeyReturn || key == unison.KeyNumPadEnter {
			e.submitInput()
			return true
		}
		return e.input.DefaultKeyDown(key, mods, repeat)
	}
	e.input.ModifiedCallback = func(before, after *unison.FieldState) {
		if len(after.Text) > dino.MaxInputBytes {
			e.input.ApplyFieldState(before)
			e.setStatus("Please enter one whole number.")
		}
	}

	panes := unison.NewPanel()
	panes.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: 12, EqualColumns: true})
	panes.SetLayoutData(fill(true))
	panes.AddChild(pane("Code", e.source, nil))
	panes.AddChild(pane("Output", e.output, e.input))
	content.AddChild(panes)

	footer := unison.NewPanel()
	footer.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: 8, VAlign: align.Middle})
	footer.SetLayoutData(fill(false))
	e.status.SetLayoutData(fill(false))
	footer.AddChild(e.status)
	footer.AddChild(e.jump)
	content.AddChild(footer)
	e.zoom(0)
}

func pane(title string, field *unison.Field, input *unison.Field) *unison.Panel {
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 6})
	panel.SetLayoutData(fill(true))

	label := unison.NewLabel()
	label.SetTitle(title)
	field.Accessibility.LabeledBy = label
	panel.AddChild(label)
	scroll := unison.NewScrollPanel()
	scroll.SetLayoutData(&unison.FlexLayoutData{
		HGrab: true, VGrab: true, HAlign: align.Fill, VAlign: align.Fill,
		SizeHint: geom.NewSize(380, 360), MinSize: geom.NewSize(220, 160),
	})
	width := behavior.Follow
	if !field.Wrap() {
		width = behavior.Fill
	}
	field.AutoScroll = false
	scroll.SetContent(field, width, behavior.HintedFill)
	panel.AddChild(scroll)
	if input != nil {
		input.SetLayoutData(fill(false))
		panel.AddChild(input)
	}
	return panel
}

func button(title string, click func()) *unison.Button {
	b := unison.NewButton()
	b.SetTitle(title)
	b.ClickCallback = click
	return b
}

func fill(vertical bool) *unison.FlexLayoutData {
	return &unison.FlexLayoutData{HGrab: true, VGrab: vertical, HAlign: align.Fill, VAlign: align.Fill}
}

func (e *editor) zoom(change float32) {
	size := float32(15)
	if change != 0 {
		size = min(max(e.source.Font.Size()+change, 11), 28)
	}
	e.source.Font = unison.MonospacedFont.Face().Font(size)
	e.output.Font = e.source.Font
	e.source.MarkForLayoutAndRedraw()
	e.output.MarkForLayoutAndRedraw()
}

func (e *editor) onKey(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
	switch {
	case key == unison.KeyF1:
		if !repeat {
			e.openTutorial()
		}
		return true
	case key == unison.KeyF5:
		if !repeat {
			e.startRun()
		}
		return true
	case key == unison.KeyEscape && e.active != nil:
		e.stopRun()
		return true
	case mods&mod.NonSticky == mod.OSMenuCommand() && !repeat:
		switch key {
		case unison.KeyO:
			e.open()
			return true
		case unison.KeyS:
			e.save()
			return true
		case unison.KeyQ:
			e.window.AttemptClose()
			return true
		case unison.KeyZ:
			e.undo.Undo()
			return true
		case unison.KeyY:
			e.undo.Redo()
			return true
		}
	case mods&mod.NonSticky == mod.OSMenuCommand()|mod.Shift && key == unison.KeyZ:
		e.undo.Redo()
		return true
	}
	return false
}

func (e *editor) codeKey(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
	if key != unison.KeyReturn && key != unison.KeyNumPadEnter || mods&mod.NonSticky != mod.None {
		return e.source.DefaultKeyDown(key, mods, repeat)
	}
	state := e.source.GetFieldState()
	before := string([]rune(state.Text)[:state.SelectionStart])
	line := before[strings.LastIndexByte(before, '\n')+1:]
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	if strings.HasPrefix(trimmed, "function ") ||
		strings.HasPrefix(trimmed, "if ") && strings.HasSuffix(trimmed, " then") ||
		strings.HasPrefix(trimmed, "while ") && strings.HasSuffix(trimmed, " do") ||
		trimmed == "else" {
		indent += "  "
	}
	if !e.source.DefaultKeyDown(key, mods, repeat) {
		return false
	}
	for range len(indent) {
		e.source.DefaultRuneTyped(' ')
	}
	return true
}

func (e *editor) edited(before, after *unison.FieldState) {
	if len(after.Text) > dino.MaxSourceBytes {
		e.source.ApplyFieldState(before)
		e.setStatus("This program is too large. Try a smaller file.")
		return
	}
	e.undo.Add(&unison.UndoEdit[*unison.FieldState]{
		ID: e.source.CurrentUndoID(), EditName: "Typing",
		BeforeData: before, AfterData: after, EditCost: len(before.Text) + len(after.Text),
		UndoFunc: func(edit *unison.UndoEdit[*unison.FieldState]) { e.applyEdit(edit.BeforeData) },
		RedoFunc: func(edit *unison.UndoEdit[*unison.FieldState]) { e.applyEdit(edit.AfterData) },
		AbsorbFunc: func(edit *unison.UndoEdit[*unison.FieldState], next unison.Undoable) bool {
			other, ok := next.(*unison.UndoEdit[*unison.FieldState])
			if ok && edit.ID == other.ID {
				edit.AfterData = other.AfterData
				edit.EditCost = len(edit.BeforeData.Text) + len(edit.AfterData.Text)
				return true
			}
			return false
		},
	})
	e.updateDocument()
}

func (e *editor) applyEdit(state *unison.FieldState) {
	e.source.ApplyFieldState(state)
	e.updateDocument()
}

func (e *editor) openExample(name, title string) {
	if e.dirty() && !e.dialogs.discardChanges() {
		return
	}
	source := example(name)
	e.load(draft{Text: source, SavedText: source})
	e.setStatus("Opened " + title)
}

func (e *editor) load(document draft) {
	e.document = document
	e.source.ApplyFieldState(&unison.FieldState{Text: document.Text})
	e.undo.Clear()
	e.window.SetFocus(e.source)
	e.updateDocument()
}

func (e *editor) updateDocument() {
	e.document.Text = e.source.Text()
	title := "Untitled"
	if e.document.Path != "" {
		title = filepath.Base(e.document.Path)
	}
	if e.dirty() {
		title += " *"
	}
	e.window.SetTitle(fmt.Sprintf("%s — Dinoscript", title))
	e.source.MarkForLayoutAndRedraw()
	e.updateJump()

	if !e.draftQueued && e.draftPath != "" {
		e.draftQueued = true
		unison.InvokeTaskAfter(func() {
			e.draftQueued = false
			if !e.closed {
				if err := e.writeDraft(); err != nil {
					e.setStatus("Couldn't recover this draft automatically. Use Save to keep your work.")
				}
			}
		}, 350*time.Millisecond)
	}
}

func (e *editor) dirty() bool { return e.source.Text() != e.document.SavedText }

func (e *editor) setStatus(message string) {
	e.status.SetTitle(message)
	e.status.MarkForLayoutAndRedraw()
	unison.AnnounceForAccessibility(message)
}
