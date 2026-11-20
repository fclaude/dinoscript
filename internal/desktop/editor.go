package desktop

import (
	"embed"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/fclaude/dinoscript/internal/locale"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
)

//go:embed examples/*.ds examples/*/*.ds
var examples embed.FS

const (
	numberMagicExample       = 5
	rockPaperScissorsExample = 6
)

var lessons = []struct {
	title     string
	file      string
	localized map[locale.Language]string
}{
	{"Hello, dinosaur!", "hello.ds", map[locale.Language]string{locale.Spanish: "hola.ds", locale.Polish: "czesc.ds"}},
	{"Make a decision", "decisions.ds", map[locale.Language]string{locale.Spanish: "decisiones.ds", locale.Polish: "decyzje.ds"}},
	{"Count with a loop", "counting.ds", map[locale.Language]string{locale.Spanish: "contar.ds", locale.Polish: "liczenie.ds"}},
	{"Fibonacci numbers", "fibonacci.ds", map[locale.Language]string{locale.Spanish: "serie-fibonacci.ds", locale.Polish: "ciag-fibonacciego.ds"}},
	{"Is it prime?", "prime.ds", map[locale.Language]string{locale.Spanish: "primo.ds", locale.Polish: "liczba-pierwsza.ds"}},
	numberMagicExample:       {"Pip Reads Your Mind", "number-magic.ds", map[locale.Language]string{locale.Spanish: "numero-secreto.ds", locale.Polish: "sekretna-liczba.ds"}},
	rockPaperScissorsExample: {"Rock, Paper, Scissors", "rock-paper-scissors.ds", map[locale.Language]string{locale.Spanish: "cachipun.ds", locale.Polish: "kamien-papier-nozyce.ds"}},
}

type editor struct {
	window       *unison.Window
	source       *unison.Field
	output       *unison.Field
	input        *unison.Field
	status       *unison.Label
	run          *unison.Button
	stop         *unison.Button
	jump         *unison.Button
	learn        *unison.Button
	tabList      *unison.Panel
	fileTitle    *unison.Label
	position     *unison.Label
	tabs         []*fileTab
	newFile      *unison.Button
	sourceScroll *unison.ScrollPanel
	outputScroll *unison.ScrollPanel
	codeView     codeDecoration
	reader       *tutorialReader
	language     locale.Language
	systemLocale locale.Language
	preference   locale.Language
	localized    []func()
	statusText   string
	statusArgs   []any

	dialogs     fileDialogs
	draftPath   string
	draftQueued bool
	document    *fileTab
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
	_, err := newEditor(path, nil, locale.System())
	return err
}

func newEditor(draftPath string, dialogs fileDialogs, systemLocale locale.Language) (*editor, error) {
	window, err := unison.NewWindow("Dinoscript")
	if err != nil {
		return nil, err
	}
	e := &editor{
		window: window, dialogs: dialogs, draftPath: draftPath, systemLocale: systemLocale,
		source: unison.NewMultiLineField(), output: unison.NewMultiLineField(),
		input: unison.NewField(), status: unison.NewLabel(),
	}
	e.loadLanguage()
	if e.dialogs == nil {
		e.dialogs = systemDialogs{editor: e}
	}
	pip, err := unison.NewImageFromBytes(pipPortrait, geom.NewPoint(1, 1))
	if err != nil {
		window.Dispose()
		return nil, err
	}
	e.configureFields()
	e.buildWindow(pip)
	window.SetTitleIcons([]*unison.Image{pip})
	e.restoreTabs()
	e.setStatus("Ready.")
	window.KeyDownCallback = e.onKey
	window.AllowCloseCallback = func() bool {
		if err := e.writeDraft(); err != nil && e.hasUnsavedTabs() {
			return e.dialogs.discardChanges()
		}
		return true
	}
	window.WillCloseCallback = func() {
		e.closed = true
		pip.Dispose()
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

func (e *editor) configureFields() {
	e.source.SetWrap(false)
	e.source.NoSelectAllOnFocus = true
	e.source.ModifiedCallback = e.edited
	e.source.KeyDownCallback = e.codeKey
	e.localize(func() {
		e.output.Watermark = e.text("Run your code to see what it prints.")
		e.output.MarkForLayoutAndRedraw()
	})
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
	e.localize(func() {
		e.input.Watermark = e.text("Program input")
		e.input.Accessibility.Name = e.input.Watermark
		e.input.MarkForLayoutAndRedraw()
	})
	e.input.SetEnabled(false)
	e.input.KeyDownCallback = func(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
		if key == unison.KeyReturn || key == unison.KeyNumPadEnter {
			if !repeat {
				e.submitInput()
			}
			return true
		}
		return e.input.DefaultKeyDown(key, mods, repeat)
	}
	e.input.ModifiedCallback = func(before, after *unison.FieldState) {
		if len(after.Text) > dino.MaxInputBytes {
			e.input.ApplyFieldState(before)
			e.setStatus("That input is too long.")
		}
	}

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
	e.refreshCode()
	e.sourceScroll.MarkForLayoutAndRedraw()
	e.outputScroll.MarkForLayoutAndRedraw()
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
		case unison.KeyN:
			e.newTab()
			return true
		case unison.KeyW:
			e.closeTab()
			return true
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
			e.document.undo.Undo()
			return true
		case unison.KeyY:
			e.document.undo.Redo()
			return true
		}
	case mods&mod.NonSticky == mod.OSMenuCommand()|mod.Shift && key == unison.KeyZ:
		e.document.undo.Redo()
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
	e.document.undo.Add(&unison.UndoEdit[*unison.FieldState]{
		ID: e.source.CurrentUndoID(), EditName: e.text("Typing"),
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

func (e *editor) updateDocument() {
	e.document.Text = e.source.Text()
	e.updateTabTitle()
	e.refreshCode()
	e.sourceScroll.MarkForLayoutAndRedraw()
	e.updateJump()
	e.queueDraft()
}

func (e *editor) queueDraft() {
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

func (e *editor) dirty() bool { return e.document.Text != e.document.SavedText }

func (e *editor) setStatus(message string, args ...any) {
	e.statusText, e.statusArgs = message, args
	e.renderStatus()
}

func (e *editor) renderStatus() {
	args := slices.Clone(e.statusArgs)
	for index, arg := range args {
		if message, ok := arg.(statusMessage); ok {
			args[index] = e.text(string(message))
		}
	}
	message := e.text(e.statusText, args...)
	e.status.SetTitle(message)
	e.status.MarkForLayoutAndRedraw()
	unison.AnnounceForAccessibility(message)
}
