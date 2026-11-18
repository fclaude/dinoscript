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
	guide, err := book.Load()
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
	reader := &tutorialReader{editor: e, guide: guide, window: window, images: make(map[string]*unison.Image)}
	e.reader = reader
	window.WillCloseCallback = func() {
		for _, image := range reader.images {
			image.Dispose()
		}
		e.reader = nil
	}
	reader.build()
	reader.showPage(0)
	window.PackWithDefaultInitialLocation()
	window.ToFront()
	window.SetFocus(reader.next)
}

func (r *tutorialReader) build() {
	content := r.window.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 10})
	content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(12)))

	r.contents = unison.NewPopupMenu[int]()
	r.contents.Accessibility.Name = "Choose a lesson"
	for number := 0; number <= len(r.guide.Pages); number++ {
		r.contents.AddItem(number)
	}
	r.contents.ItemRendererCallback = func(number int) string {
		if number == 0 {
			return "Cover"
		}
		return fmt.Sprintf("%d. %s", number, r.guide.Pages[number-1].Title)
	}
	r.contents.ChoiceMadeCallback = func(_ *unison.PopupMenu[int], _, page int) { r.showPage(page) }
	r.previous = button("Previous", func() { r.showPage(r.page - 1) })
	r.next = button("Next", func() { r.showPage(r.page + 1) })
	r.tryCode = button("Try code", r.loadCode)
	r.save = button("Save PDF…", r.savePDF)

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

	if number == 0 {
		r.addText(fmt.Sprintf("# %s\n\n%s\n\nBy %s", r.guide.Title, r.guide.Subtitle, r.guide.Author))
		r.addArt(r.guide.CoverArt, r.guide.CoverAlt)
		r.addText(r.guide.Welcome + "\n\nChoose **Next** to begin, or pick a lesson from the list.")
		r.setStatus("Cover")
	} else {
		lesson := r.guide.Pages[number-1]
		r.addText(fmt.Sprintf("**%s**\n\n# %s\n\n> %s\n\n%s",
			lesson.Chapter, lesson.Title, lesson.Pip, strings.Join(lesson.Body, "\n\n")))
		r.addArt(lesson.Art, lesson.Alt)
		r.addText("```text\n" + lesson.Code + "```\n\n## Try it\n\n" + lesson.Challenge + "\n\n## Pip's hint\n\n" + lesson.Hint)
		r.setStatus(fmt.Sprintf("Lesson %d of %d", number, len(r.guide.Pages)))
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
		e.openSample(fmt.Sprintf("Lesson %02d.ds", r.page), lesson.Code)
	}
	e.window.ToFront()
	e.window.SetFocus(e.source)
	e.setStatus("Press F5 to run your lesson.")
}

func (r *tutorialReader) savePDF() {
	path, ok := r.editor.dialogs.savePath("pdf", book.PDFFilename)
	if !ok {
		return
	}
	pdf, err := book.PDF()
	if err == nil {
		err = writeFile(path, pdf)
	}
	if err != nil {
		r.setStatus("Couldn't save the PDF.")
		return
	}
	r.setStatus("Saved " + filepath.Base(path) + ".")
}

func (r *tutorialReader) setStatus(message string) {
	r.status.SetTitle(message)
	r.status.MarkForLayoutAndRedraw()
	unison.AnnounceForAccessibility(message)
}
