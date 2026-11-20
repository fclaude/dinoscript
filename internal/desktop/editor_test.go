package desktop

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fclaude/dinoscript/book"
	"github.com/fclaude/dinoscript/internal/locale"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/mod"
)

func TestDesktopEditor(t *testing.T) {
	dir := t.TempDir()
	draftPath := filepath.Join(dir, "draft.json")
	dialogs := &testDialogs{
		save: filepath.Join(dir, "saved.ds"),
		open: filepath.Join(dir, "opened.ds"),
		pdf:  filepath.Join(dir, book.PDFFilename),
	}
	if err := os.WriteFile(dialogs.open, []byte("print(\"Reopened\")\r\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var app *editor
	var startupErr error
	screen, err := unison.StartHeadless(
		unison.HeadlessConfig{Width: 1024, Height: 768, SyncTimeout: 5 * time.Second},
		unison.QuitAfterLastWindowClosedCallback(func() bool { return false }),
		unison.StartupFinishedCallback(func() { app, startupErr = newEditor(draftPath, dialogs, locale.English) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		screen.Stop()
		if errors := screen.Errors(); len(errors) != 0 {
			t.Errorf("UI event errors: %v", errors)
		}
	})
	if startupErr != nil || app == nil {
		t.Fatalf("create editor: %v", startupErr)
	}
	// Load the system fallback font outside the input driver's redraw deadline.
	// The test below still types the emoji, renders it and checks its diagnostic offset.
	screen.Do(func() { unison.NewText("🦖", &unison.TextDecoration{Font: app.source.Font}) })
	screen.EnableAccessibility()

	assert := func(description string, condition func() bool) {
		t.Helper()
		var ok bool
		if !screen.Do(func() { ok = condition() }) || !ok {
			t.Fatalf("%s", description)
		}
	}
	wait := func(status string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		var got string
		for time.Now().Before(deadline) {
			if screen.Do(func() { got = app.status.String() }) && strings.HasPrefix(got, status) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("waiting for status %q, got %q", status, got)
	}
	click := func(button *unison.Button) { screen.Click(screen.PanelCenter(button)) }
	replaceCode := func(source string) {
		screen.Click(screen.PanelCenter(app.source))
		screen.KeyPress(unison.KeyA, mod.Control)
		screen.Type(source)
	}
	run := func() { screen.KeyPress(unison.KeyF5, mod.None) }
	playMagic := func(pauseStatus, numberStatus, finishedStatus, reveal string, prompts ...string) {
		t.Helper()
		click(app.tabs[3].button)
		run()
		for _, prompt := range prompts {
			wait(pauseStatus)
			assert("the game should wait for Enter after each instruction", func() bool {
				return app.input.Enabled() && app.input.Text() == "" && strings.HasSuffix(app.output.Text(), prompt+"\n")
			})
			screen.KeyPress(unison.KeyReturn, mod.None)
		}
		wait(numberStatus)
		screen.Type("20")
		screen.KeyPress(unison.KeyReturn, mod.None)
		wait(finishedStatus)
		assert("the game should reveal the secret number after the final answer", func() bool {
			return strings.HasSuffix(app.output.Text(), "> 20\n"+reveal+"\n")
		})
	}

	assert("the source field should have startup focus", func() bool { return app.window.CurrentFocus() == app.source.AsPanel() })
	assert("the editor should open with Pip, Hello, and the number trick", func() bool {
		return len(app.window.TitleIcons()) == 1 && len(app.tabs) == 4 &&
			app.tabTitle(app.document) == "hello.ds" && app.tabTitle(app.tabs[3]) == "number-magic.ds"
	})
	click(app.run)
	wait("Finished.")
	assert("Run should show the bundled example's output", func() bool {
		return app.output.Text() == "Hello, dinosaur!\n"
	})
	if node := screen.AccessibilityNodeFor(app.output); node == nil || !node.ReadOnly {
		t.Fatal("Output should be exposed to assistive technology as read-only")
	}
	screen.Click(screen.PanelCenter(app.output))
	screen.KeyPress(unison.KeyA, mod.Control)
	screen.Type("Ignored")
	screen.KeyPress(unison.KeyBackspace, mod.None)
	assert("typing and deleting in Output should leave the program's text intact", func() bool {
		return app.output.Text() == "Hello, dinosaur!\n"
	})

	outputScroll := app.output.ScrollRoot()
	var outputHeight float32
	var windowRect geom.Rect
	screen.Do(func() {
		outputHeight = outputScroll.FrameRect().Height
		windowRect = app.window.ContentRect()
		taller := windowRect
		taller.Height += 100
		app.window.SetContentRect(taller)
	})
	assert("Output should grow when the window gets taller", func() bool {
		return outputScroll.FrameRect().Height > outputHeight
	})
	screen.Do(func() { app.window.SetContentRect(windowRect) })
	replaceCode(`let count = 1
while count <= 120 do
  print("Dinosaur ", count)
  let count = count + 1
end while
pause()
print(121)`)
	run()
	wait("Press Enter to continue.")
	outputBar := outputScroll.Bar(false)
	assert("long output should show a scroll bar and follow the latest line", func() bool {
		return strings.HasSuffix(app.output.Text(), "Dinosaur 120\n") &&
			outputBar.Thumb().Height > 0 && outputBar.MaxValue() > 0 && outputBar.Value() == outputBar.MaxValue()
	})
	screen.Wheel(screen.PanelCenter(outputScroll.ContentView()), geom.NewPoint(0, 2), mod.None)
	var readingPosition float32
	assert("the mouse wheel should scroll back through the output", func() bool {
		readingPosition = outputBar.Value()
		return readingPosition < outputBar.MaxValue()
	})
	screen.KeyPress(unison.KeyReturn, mod.None)
	wait("Finished.")
	assert("new output should preserve the scroll position while reading earlier lines", func() bool {
		return strings.HasSuffix(app.output.Text(), "Dinosaur 120\n121\n") && outputBar.Value() == readingPosition
	})
	screen.Wheel(screen.PanelCenter(outputScroll.ContentView()), geom.NewPoint(0, 200), mod.None)
	assert("the first line should still be reachable", func() bool {
		return strings.HasPrefix(app.output.Text(), "Dinosaur 1\n") && outputBar.Value() == 0
	})

	playMagic("Press Enter to continue.", "Enter a whole number", "Finished.", "Your secret number was 7!",
		"Press Enter when you're ready.", "Multiply it by 2, then press Enter.", "Add 10, then press Enter.", "Subtract 4, then press Enter.")
	click(app.tabs[0].button)
	replaceCode("print(\"🦖\", missing)")
	run()
	wait("Fix the error")
	assert("the output pane should explain the unknown name", func() bool {
		return strings.Contains(app.output.Text(), "I don't know \"missing\"")
	})
	if !slices.ContainsFunc(screen.Announcements(), func(text string) bool { return strings.Contains(text, "I don't know") }) {
		t.Fatal("the source diagnostic was not announced for accessibility")
	}
	click(app.jump)
	assert("Go to error should select the exact identifier after the Unicode string", func() bool {
		return app.window.CurrentFocus() == app.source.AsPanel() && app.source.SelectedText() == "missing"
	})
	screen.Type("2 + 3 * 4")
	run()
	wait("Finished.")
	assert("F5 should run the repaired source", func() bool { return app.output.Text() == "🦖14\n" })

	replaceCode("print(\"Number?\")\nprint(readint() + 1)")
	run()
	wait("Enter a whole number")
	assert("program input should follow the prompt and take focus", func() bool {
		return app.output.Text() == "Number?\n" && app.window.CurrentFocus() == app.input.AsPanel()
	})
	screen.Type("41")
	screen.KeyPress(unison.KeyReturn, mod.None)
	wait("Finished.")
	assert("Enter should send the response back to the program", func() bool {
		return app.output.Text() == "Number?\n> 41\n42\n"
	})
	replaceCode("print(readint() / 0)")
	run()
	wait("Enter a whole number")
	replaceCode("print(9)")
	screen.Click(screen.PanelCenter(app.input))
	screen.Type("1")
	screen.KeyPress(unison.KeyReturn, mod.None)
	wait("Fix the error")
	assert("diagnostics from old source should stay visible without selecting the new source", func() bool {
		return strings.Contains(app.output.Text(), "divide by zero") && !app.jump.Enabled()
	})

	replaceCode("print(readint())")
	run()
	wait("Enter a whole number")
	click(app.stop)
	wait("Stopped.")
	assert("Stop should release the waiting run and disable its input", func() bool {
		return app.run.Enabled() && !app.input.Enabled()
	})

	replaceCode("print(7)")
	screen.KeyPress(unison.KeyZ, mod.Control)
	assert("Undo should restore the previous source", func() bool { return app.source.Text() == "print(readint())" })
	screen.KeyPress(unison.KeyY, mod.Control)
	assert("Redo should restore the edit", func() bool { return app.source.Text() == "print(7)" })
	screen.KeyPress(unison.KeyS, mod.Control)
	wait("Saved")
	data, err := os.ReadFile(dialogs.save)
	if err != nil || string(data) != "print(7)" {
		t.Fatalf("saved contents = %q, error = %v", data, err)
	}
	replaceCode("print(8)")
	var editedTab *fileTab
	screen.Do(func() { editedTab = app.document })
	screen.KeyPress(unison.KeyO, mod.Control)
	wait("Opened")
	assert("Open should keep pending edits and normalize Windows newlines in the new tab", func() bool {
		return editedTab.Text == "print(8)" && editedTab.SavedText == "print(7)" && app.source.Text() == "print(\"Reopened\")\n" && dialogs.openCount == 1
	})
	var opened *fileTab
	screen.Do(func() { opened = app.document })
	click(editedTab.button)
	screen.KeyPress(unison.KeyZ, mod.Control)
	assert("undo belongs to the selected tab", func() bool { return app.source.Text() == "print(7)" && opened.Text == "print(\"Reopened\")\n" })
	screen.KeyPress(unison.KeyY, mod.Control)
	click(opened.button)
	run()
	wait("Finished.")
	assert("a fresh run after Stop should work", func() bool { return app.output.Text() == "Reopened\n" })

	replaceCode("print(\"Recover me\")")
	screen.Do(func() {
		if !app.window.AttemptClose() {
			return
		}
		app, startupErr = newEditor(draftPath, dialogs, locale.English)
	})
	if startupErr != nil {
		t.Fatal(startupErr)
	}
	assert("reopening the window should recover the unsaved draft and its file path", func() bool {
		return app.source.Text() == "print(\"Recover me\")" && app.dirty() && app.document.Path == dialogs.open &&
			app.tabs[0].Text == "print(8)" && app.tabs[0].SavedText == "print(7)"
	})
	click(app.newFile)
	replaceCode("print(readint())")
	run()
	wait("Enter a whole number")
	screen.Do(func() { opened = app.tabs[len(app.tabs)-2] })
	click(opened.button)
	assert("a tab switch should release the waiting run and keep output attached to the current file", func() bool {
		return app.active == nil && !app.input.Enabled() && app.output.Text() == "" && app.source.Text() == "print(\"Recover me\")"
	})
	screen.KeyPress(unison.KeyW, mod.Control)
	assert("declining to close an edited tab should keep it open", func() bool { return app.document == opened })

	click(app.learn)
	var reader *tutorialReader
	assert("Learn should open the illustrated cover in a separate window", func() bool {
		reader = app.reader
		return reader != nil && reader.page == 0 && len(reader.images) == 1 && !reader.tryCode.Enabled()
	})
	click(reader.next)
	click(reader.next)
	click(reader.previous)
	assert("Next and Previous should move one lesson at a time", func() bool {
		return reader.page == 1 && reader.status.String() == fmt.Sprintf("Lesson 1 of %d", len(reader.guide.Pages))
	})
	click(reader.tryCode)
	assert("Try code should open the lesson separately and give the editor focus", func() bool {
		return opened.Text == "print(\"Recover me\")" && app.source.Text() == reader.guide.Pages[0].Code && app.window.Focused() && app.window.CurrentFocus() == app.source.AsPanel()
	})
	run()
	wait("Finished.")
	assert("the tutorial code should run in the ordinary editor", func() bool {
		return app.output.Text() == "Hello, dinosaur!\n"
	})
	click(app.learn)
	assert("returning to Learn should keep the current lesson", func() bool {
		return app.reader == reader && reader.page == 1
	})
	click(reader.save)
	printed, err := os.ReadFile(dialogs.pdf)
	if err != nil {
		t.Fatal(err)
	}
	original, err := book.PDF()
	if err != nil || !bytes.Equal(printed, original) {
		t.Fatalf("Save PDF did not export the bundled book: %v", err)
	}

	languageMenu := func() *unison.PopupMenu[locale.Language] {
		var languages *unison.PopupMenu[locale.Language]
		assert("the editor should expose a language menu", func() bool {
			app.window.ToFront()
			for _, child := range app.window.Content().Children()[0].Children() {
				if menu, ok := child.Self.(*unison.PopupMenu[locale.Language]); ok {
					languages = menu
					return true
				}
			}
			return false
		})
		return languages
	}
	languages := languageMenu()
	replaceCode("print(\"Pip\", missing)")
	run()
	wait("Fix the error")
	screen.Click(screen.PanelCenter(languages))
	for range languages.ItemCount() {
		screen.KeyPress(unison.KeyDown, mod.None)
	}
	screen.KeyPress(unison.KeyUp, mod.None)
	screen.KeyPress(unison.KeyReturn, mod.None)
	assert("choosing Spanish should translate the editor and diagnostic while preserving code and the open lesson", func() bool {
		return app.language == locale.Spanish && app.source.Text() == "print(\"Pip\", missing)" && app.dirty() &&
			app.run.Text.String() == "Ejecutar  F5" && app.source.Accessibility.Name == "Código" &&
			strings.Contains(app.output.Text(), "Línea 1, columna 14: Todavía no conozco \"missing\"") &&
			app.jump.Enabled() && app.reader == reader && reader.page == 1 && reader.status.String() == fmt.Sprintf("Lección 1 de %d", len(reader.guide.Pages))
	})
	click(app.learn)
	click(reader.save)
	printed, err = os.ReadFile(dialogs.pdf)
	if err != nil {
		t.Fatal(err)
	}
	original, err = book.PDFLanguage(locale.Spanish)
	if err != nil || !bytes.Equal(printed, original) {
		t.Fatalf("Guardar PDF did not export the Spanish edition: %v", err)
	}
	click(reader.tryCode)
	run()
	wait("Ejecución terminada.")
	assert("the Spanish lesson code should run in the ordinary editor", func() bool {
		return app.output.Text() == "¡Hola, dinosaurio!\n"
	})
	screen.Do(func() {
		if app.window.AttemptClose() {
			app, startupErr = newEditor(draftPath, dialogs, locale.English)
		}
	})
	if startupErr != nil {
		t.Fatal(startupErr)
	}
	assert("an explicit language choice should survive a restart", func() bool {
		return app.language == locale.Spanish && app.source.Text() == "print(\"¡Hola, dinosaurio!\")\n"
	})
	screen.Do(func() { app.changeLanguage(systemLanguage) })
	assert("returning to the system language should restore the English interface", func() bool {
		return app.language == locale.English && app.run.Text.String() == "Run  F5"
	})
	freshDraft := filepath.Join(t.TempDir(), "draft.json")
	screen.Do(func() {
		if app.window.AttemptClose() {
			app, startupErr = newEditor(freshDraft, dialogs, locale.Spanish)
		}
	})
	if startupErr != nil {
		t.Fatal(startupErr)
	}
	assert("a fresh installation on a Spanish system should open with Spanish examples", func() bool {
		return app.preference == systemLanguage && app.language == locale.Spanish &&
			app.tabTitle(app.document) == "hola.ds" && strings.Contains(app.source.Text(), "¡Hola, dinosaurio!") &&
			len(app.tabs) == 4 && app.tabTitle(app.tabs[3]) == "numero-secreto.ds"
	})
	playMagic("Presiona Enter para continuar.", "Ingresa un número entero", "Ejecución terminada.", "¡Tu número secreto era 7!",
		"Presiona Enter cuando estés listo.", "Multiplícalo por 2 y presiona Enter.", "Súmale 10 y presiona Enter.", "Réstale 4 y presiona Enter.")

	languages = languageMenu()
	replaceCode("print(\"Pip\", missing)")
	run()
	wait("Corrige el error")
	screen.Click(screen.PanelCenter(languages))
	for range languages.ItemCount() {
		screen.KeyPress(unison.KeyDown, mod.None)
	}
	screen.KeyPress(unison.KeyReturn, mod.None)
	assert("choosing Polish should translate the editor and diagnostic while preserving the source", func() bool {
		return app.language == locale.Polish && app.run.Text.String() == "Uruchom  F5" &&
			app.source.Text() == "print(\"Pip\", missing)" &&
			strings.Contains(app.output.Text(), "Wiersz 1, kolumna 14: Nie znam jeszcze nazwy \"missing\"")
	})
	click(app.learn)
	assert("Learn should open the Polish workbook", func() bool {
		return app.reader != nil && app.reader.guide.Title == "Dinoscript z Pip"
	})
}

type testDialogs struct {
	open, save string
	pdf        string
	discard    bool
	openCount  int
}

func (d *testDialogs) openPath() (string, bool) {
	d.openCount++
	return d.open, true
}

func (d *testDialogs) savePath(extension, _ string) (string, bool) {
	if extension == "pdf" {
		return d.pdf, d.pdf != ""
	}
	return d.save, d.save != ""
}

func (d *testDialogs) discardChanges() bool { return d.discard }

func TestReadSingleDocumentDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.json")
	if err := os.WriteFile(path, []byte(`{"Text":"print(8)","Path":"saved.ds","SavedText":"print(7)"}`), 0600); err != nil {
		t.Fatal(err)
	}
	session, ok := readWorkspace(path)
	if !ok || session.Active != 0 || len(session.Tabs) != 1 {
		t.Fatalf("recover legacy workspace: %+v", session)
	}
	want := draft{Text: "print(8)", Path: "saved.ds", SavedText: "print(7)"}
	if session.Tabs[0] != want {
		t.Errorf("recovered tab = %+v, want %+v", session.Tabs[0], want)
	}
}
