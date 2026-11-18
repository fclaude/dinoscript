package desktop

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fclaude/dinoscript/book"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
)

type tutorialReader struct {
	editor *editor
	guide  book.Guide
	page   int
	window *unison.Window
	scroll *unison.ScrollPanel
	body   *unison.Panel

	contents *unison.PopupMenu[int]
	previous *unison.Button
	next     *unison.Button
	tryCode  *unison.Button
	save     *unison.Button
	status   *unison.Label
	images   map[string]*unison.Image
}

func (e *editor) openTutorial() {
	if e.reader != nil {
		e.reader.window.ToFront()
		return
	}
	guide, err := book.LoadLanguage(e.language)
	if err != nil {
		e.setStatus("Couldn't open the Dinoscript book.")
		return
	}
	window, err := unison.NewWindow(guide.Title)
	if err != nil {
		e.setStatus("Couldn't open the Dinoscript book.")
		return
	}
	window.SetTitleIcons(e.window.TitleIcons())
	reader := &tutorialReader{editor: e, window: window, images: make(map[string]*unison.Image)}
	e.reader = reader
	window.WillCloseCallback = func() {
		for _, image := range reader.images {
			image.Dispose()
		}
		e.reader = nil
	}
	reader.build()
	reader.showGuide(guide)
	window.PackWithDefaultInitialLocation()
	window.ToFront()
	window.SetFocus(reader.next)
}

func (r *tutorialReader) build() {
	content := r.window.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 10})
	content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(12)))

	r.contents = unison.NewPopupMenu[int]()
	r.contents.ItemRendererCallback = func(number int) string {
		if number == 0 {
			return r.editor.text("Cover")
		}
		return fmt.Sprintf("%d. %s", number, r.guide.Pages[number-1].Title)
	}
	r.contents.ChoiceMadeCallback = func(_ *unison.PopupMenu[int], _, page int) { r.showPage(page) }
	r.previous = button("", func() { r.showPage(r.page - 1) })
	r.next = button("", func() { r.showPage(r.page + 1) })
	r.tryCode = button("", r.loadCode)
	r.save = button("", r.savePDF)

	navigation := unison.NewPanel()
	navigation.SetLayout(&unison.FlexLayout{Columns: 3, HSpacing: 6, VAlign: align.Middle})
	navigation.AddChild(r.previous)
	navigation.AddChild(r.next)
	navigation.AddChild(r.contents)
	content.AddChild(navigation)

	r.scroll = unison.NewScrollPanel()
	r.scroll.SetLayoutData(&unison.FlexLayoutData{
		HGrab: true, VGrab: true, HAlign: align.Fill, VAlign: align.Fill,
		SizeHint: geom.NewSize(535, 485), MinSize: geom.NewSize(360, 210),
	})
	r.body = unison.NewPanel()
	r.body.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 10})
	r.body.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(8)))
	r.scroll.SetContent(r.body, behavior.Follow, behavior.HintedFill)
	content.AddChild(r.scroll)

	bar := unison.NewPanel()
	bar.SetLayout(&unison.FlexLayout{Columns: 3, HSpacing: 7, VAlign: align.Middle})
	bar.SetLayoutData(fill(false))
	r.status = unison.NewLabel()
	r.status.SetLayoutData(fill(false))
	bar.AddChild(r.tryCode)
	bar.AddChild(r.save)
	bar.AddChild(r.status)
	content.AddChild(bar)

	r.window.KeyDownCallback = func(key unison.KeyCode, mods mod.Modifiers, _ bool) bool {
		if mods&mod.NonSticky == mod.OSMenuCommand() && key == unison.KeyW {
			r.window.AttemptClose()
			return true
		}
		return false
	}
}

func (r *tutorialReader) refreshLanguage() {
	guide, err := book.LoadLanguage(r.editor.language)
	if err != nil {
		r.setStatus("Couldn't open the Dinoscript book.")
		return
	}
	r.showGuide(guide)
}

func (r *tutorialReader) showGuide(guide book.Guide) {
	e := r.editor
	r.guide = guide
	r.window.SetTitle(guide.Title)
	r.contents.Accessibility.Name = e.text("Choose a lesson")
	r.contents.RemoveAllItems()
	for number := 0; number <= len(guide.Pages); number++ {
		r.contents.AddItem(number)
	}
	for _, control := range []struct {
		button *unison.Button
		title  string
	}{{r.previous, "Previous"}, {r.next, "Next"}, {r.tryCode, "Try code"}, {r.save, "Save PDF…"}} {
		control.button.SetTitle(e.text(control.title))
		control.button.MarkForLayoutAndRedraw()
	}
	r.showPage(min(r.page, len(guide.Pages)))
}

func (r *tutorialReader) showPage(number int) {
	if number < 0 || number > len(r.guide.Pages) {
		return
	}
	r.page = number
	r.contents.Select(number)
	r.previous.SetEnabled(number > 0)
	r.next.SetEnabled(number < len(r.guide.Pages))
	r.tryCode.SetEnabled(number > 0)
	r.body.RemoveAllChildren()
	text := r.editor.text

	if number == 0 {
		r.addText(fmt.Sprintf("# %s\n\n%s\n\n%s", r.guide.Title, r.guide.Subtitle, text("By %s", r.guide.Author)))
		r.addArt(r.guide.CoverArt, r.guide.CoverAlt)
		r.addText(r.guide.Welcome + "\n\n" + text("Choose **Next** to begin, or pick a lesson from the list."))
		r.setStatus("Cover")
	} else {
		lesson := r.guide.Pages[number-1]
		r.addText(fmt.Sprintf("**%s**\n\n# %s\n\n> %s\n\n%s",
			lesson.Chapter, lesson.Title, lesson.Pip, strings.Join(lesson.Body, "\n\n")))
		r.addArt(lesson.Art, lesson.Alt)
		r.addText("```text\n" + lesson.Code + "```\n\n## " + text("Try it") + "\n\n" + lesson.Challenge + "\n\n## " + text("Pip's hint") + "\n\n" + lesson.Hint)
		r.setStatus("Lesson %d of %d", number, len(r.guide.Pages))
	}
	r.body.MarkForLayoutAndRedraw()
	r.scroll.SetPosition(0, 0)
}

func (r *tutorialReader) addText(contents string) {
	text := unison.NewMarkdown(true)
	text.SetFocusable(true)
	text.SetLayoutData(fill(false))
	text.SetContent(contents, 500)
	r.body.AddChild(text)
}

func (r *tutorialReader) addArt(name, description string) {
	if name == "" {
		return
	}
	image := r.images[name]
	if image == nil {
		data, err := book.Art(name)
		if err != nil {
			return
		}
		image, err = unison.NewImageFromBytes(data, geom.NewPoint(1, 1))
		if err != nil {
			return
		}
		r.images[name] = image
	}
	art := unison.NewDrawablePanel()
	art.Drawable = image
	art.Accessibility.Name = description
	width := float32(330)
	size := geom.NewSize(width, width*image.Size().Height/image.Size().Width)
	art.SetLayoutData(&unison.FlexLayoutData{SizeHint: size, HAlign: align.Start})
	r.body.AddChild(art)
}

func (r *tutorialReader) loadCode() {
	if r.page == 0 {
		return
	}
	lesson := r.guide.Pages[r.page-1]
	e := r.editor
	if e.source.Text() != lesson.Code {
		e.openSample(e.text("Lesson %02d.ds", r.page), lesson.Code)
	}
	e.window.ToFront()
	e.window.SetFocus(e.source)
	e.setStatus("Press F5 to run your lesson.")
}

func (r *tutorialReader) savePDF() {
	language := r.editor.language
	path, ok := r.editor.dialogs.savePath("pdf", book.Filename(language))
	if !ok {
		return
	}
	pdf, err := book.PDFLanguage(language)
	if err == nil {
		err = writeFile(path, pdf)
	}
	if err != nil {
		r.setStatus("Couldn't save the PDF.")
		return
	}
	r.setStatus("Saved %s.", filepath.Base(path))
}

func (r *tutorialReader) setStatus(message string, args ...any) {
	message = r.editor.text(message, args...)
	r.status.SetTitle(message)
	r.status.MarkForLayoutAndRedraw()
	unison.AnnounceForAccessibility(message)
}
