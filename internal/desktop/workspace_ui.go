package desktop

import (
	_ "embed"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// pipPortrait is a close crop of Pip from the workbook cover.
//
//go:embed pip.png
var pipPortrait []byte

const (
	cream unison.Color = 0xfffffcf2
	paper unison.Color = 0xfffffefa
	shelf unison.Color = 0xfff1e8cb
	sage  unison.Color = 0xffeef3e9
	ink   unison.Color = 0xff25364b
	muted unison.Color = 0xff58645f
	edge  unison.Color = 0xffc8c5b7
	focus unison.Color = 0xff365d87
	coral unison.Color = 0xffc34630
)

var (
	tabColors = [...]struct{ face, stripe unison.Color }{
		{0xffffc2a0, 0xffb44b2c}, {0xffacdfd7, 0xff2d776e}, {0xffd5c6ee, 0xff7760a0},
		{0xfff8d680, 0xff9b7028}, {0xffb8d7ee, 0xff487697},
	}
)

func surface(color unison.Color, cols int, padding geom.Insets) *unison.Panel {
	p := unison.NewPanel()
	p.SetLayout(&unison.FlexLayout{Columns: cols, HSpacing: 8, VAlign: align.Middle})
	p.SetBorder(unison.NewEmptyBorder(padding))
	p.DrawCallback = func(c *unison.Canvas, _ geom.Rect) {
		r := p.ContentRect(true)
		c.DrawRect(r, color.Paint(c, r, paintstyle.Fill))
	}
	return p
}

func label(title string, font unison.Font, color unison.Color) *unison.Label {
	l := unison.NewLabel()
	l.Font = font
	l.OnBackgroundInk = color
	l.SetTitle(title)
	return l
}

func smallIcon(svg *unison.SVG) *unison.DrawableSVG {
	return &unison.DrawableSVG{SVG: svg, Size: geom.NewUniformSize(15)}
}

func toolButton(title string, svg *unison.SVG, click func()) *unison.Button {
	b := unison.NewButton()
	b.BackgroundInk = cream
	b.OnBackgroundInk = ink
	b.EdgeInk = edge
	b.SelectionInk = focus
	b.OnSelectionInk = unison.White
	b.CornerRadius = geom.NewUniformSize(7)
	b.VMargin, b.HMargin = 5, 8
	b.Font = unison.EmphasizedSystemFont.Face().Font(13)
	if svg != nil {
		b.Drawable = smallIcon(svg)
	}
	b.SetTitle(title)
	b.ClickCallback = click
	return b
}

func (e *editor) buildWindow(pip *unison.Image) {
	content := e.window.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})
	header := surface(cream, 5, geom.NewSymmetricInsets(18, 10))
	header.SetLayoutData(fill(false))
	brand := label("Dinoscript", unison.EmphasizedSystemFont.Face().Font(28), ink)
	brand.Drawable = &unison.SizedDrawable{Drawable: pip, Size: geom.NewUniformSize(42)}
	brand.Gap = 10
	brand.SetLayoutData(fill(false))
	header.AddChild(brand)
	header.AddChild(e.examplesMenu())
	e.learn = toolButton("Learn", nil, e.openTutorial)
	header.AddChild(e.learn)
	for _, z := range []struct {
		text, name string
		step       float32
	}{{"A−", "Smaller text", -1}, {"A+", "Larger text", 1}} {
		b := toolButton(z.text, nil, func() { e.zoom(z.step) })
		b.Accessibility.Name = z.name
		b.Tooltip = unison.NewTooltipWithText(z.name)
		header.AddChild(b)
	}
	content.AddChild(header)

	body := unison.NewPanel()
	body.SetLayout(&unison.FlexLayout{Columns: 2})
	body.SetLayoutData(fill(true))
	body.AddChild(e.sidebar())
	body.AddChild(e.workspace())
	content.AddChild(body)

	footer := surface(cream, 3, geom.NewSymmetricInsets(16, 5))
	footer.SetLayoutData(fill(false))
	e.status.Font = unison.SystemFont.Face().Font(12)
	e.status.OnBackgroundInk = muted
	e.status.SetLayoutData(fill(false))
	footer.AddChild(e.status)
	e.jump = toolButton("Go to error", nil, e.jumpToError)
	e.jump.SetEnabled(false)
	e.position = label("Ln 1, Col 1", unison.MonospacedFont.Face().Font(12), muted)
	footer.AddChild(e.position)
	content.AddChild(footer)
	e.zoom(0)
}

func (e *editor) examplesMenu() *unison.PopupMenu[string] {
	m := unison.NewPopupMenu[string]()
	m.Accessibility.Name = "Examples"
	m.BackgroundInk, m.OnBackgroundInk, m.EdgeInk = cream, ink, edge
	m.CornerRadius = geom.NewUniformSize(7)
	m.VMargin = 5
	m.AddDisabledItem("Examples")
	m.Select("Examples")
	for _, lesson := range lessons {
		m.AddItem(lesson.title)
	}
	m.ChoiceMadeCallback = func(_ *unison.PopupMenu[string], _ int, title string) {
		for _, lesson := range lessons {
			if lesson.title == title {
				e.openExample(lesson.file, title)
				return
			}
		}
	}
	return m
}

func (e *editor) sidebar() *unison.Panel {
	s := surface(shelf, 1, geom.Insets{Left: 12, Top: 18, Right: 8, Bottom: 12})
	s.SetLayoutData(&unison.FlexLayoutData{VGrab: true, VAlign: align.Fill, SizeHint: geom.NewSize(205, 0)})
	s.AddChild(label("My files", unison.EmphasizedSystemFont.Face().Font(16), ink))
	e.tabList = surface(shelf, 1, geom.Insets{Top: 12, Bottom: 6})
	e.tabList.SetLayout(&unison.FlexLayout{Columns: 1, VSpacing: 7})
	scroll := unison.NewScrollPanel()
	scroll.BackgroundInk = shelf
	scroll.SetLayoutData(fill(true))
	scroll.SetContent(e.tabList, behavior.Follow, behavior.Unmodified)
	s.AddChild(scroll)
	actions := surface(shelf, 3, geom.Insets{Top: 8})
	actions.SetLayout(&unison.FlexLayout{Columns: 3, HSpacing: 4})
	e.newFile = toolButton("New", unison.CircledAddSVG, e.newTab)
	for _, b := range []*unison.Button{e.newFile, toolButton("Open", folderIcon, e.open), toolButton("Save", saveIcon, e.save)} {
		b.Side = side.Top
		b.HMargin, b.VMargin, b.Gap = 5, 4, 3
		b.SetLayoutData(fill(false))
		actions.AddChild(b)
	}
	actions.SetLayoutData(fill(false))
	s.AddChild(actions)
	return s
}

func (e *editor) fileButton(t *fileTab) *unison.Button {
	b := toolButton("", unison.DocumentSVG, func() { e.selectTab(t) })
	b.Font = unison.MonospacedFont.Face().Font(13)
	b.HAlign = align.Start
	b.VMargin, b.HMargin = 9, 11
	b.SetLayoutData(fill(false))
	b.BackgroundInk = tabColors[t.Color].face
	b.SetTitle(shortName(t.title(), 17))
	b.Accessibility.Callback = func(node *accessibility.Node) { node.Selected = e.document == t }
	b.DrawOverCallback = func(c *unison.Canvas, _ geom.Rect) {
		r := b.ContentRect(true).Inset(geom.NewSymmetricInsets(1, 6))
		r.Width = 3
		if t == e.document {
			r.Width = 5
		}
		c.DrawRoundedRect(r, geom.NewUniformSize(2), tabColors[t.Color].stripe.Paint(c, r, paintstyle.Fill))
	}
	return b
}

func (e *editor) styleTab(t *fileTab) {
	b := t.button
	title := t.title()
	full := title
	if t.Path != "" {
		full = t.Path
	}
	if t.SavedText != t.Text {
		full += " (edited)"
		title += " *"
	}
	if b.Accessibility.Name != full {
		b.Accessibility.Name = full
		b.Tooltip = unison.NewTooltipWithText(full)
		b.SetTitle(shortName(title, 17))
	}
	b.Drawable = smallIcon(unison.DocumentSVG)
	b.EdgeInk = edge
	if t == e.document {
		b.Drawable = smallIcon(unison.CheckmarkSVG)
		b.EdgeInk = tabColors[t.Color].stripe
	}
	b.MarkForLayoutAndRedraw()
}

func (e *editor) workspace() *unison.Panel {
	work := surface(paper, 1, geom.Insets{})
	work.SetLayoutData(fill(true))
	top := surface(paper, 4, geom.NewSymmetricInsets(14, 8))
	top.SetLayoutData(fill(false))
	e.fileTitle = label("Untitled.ds", unison.MonospacedFont.Face().Font(15), ink)
	e.fileTitle.Drawable = smallIcon(unison.DocumentSVG)
	e.fileTitle.SetLayoutData(fill(false))
	top.AddChild(e.fileTitle)
	closeFile := toolButton("×", nil, e.closeTab)
	closeFile.Accessibility.Name = "Close file"
	closeFile.Tooltip = unison.NewTooltipWithText("Close file")
	top.AddChild(closeFile)
	e.run = toolButton("Run  F5", playIcon, e.startRun)
	e.run.BackgroundInk, e.run.OnBackgroundInk, e.run.EdgeInk = coral, unison.White, coral
	top.AddChild(e.run)
	e.stop = toolButton("Stop", stopIcon, e.stopRun)
	e.stop.SetEnabled(false)
	top.AddChild(e.stop)
	work.AddChild(top)
	e.source.Accessibility.Name = "Code"
	e.sourceScroll = fieldScroll(e.source, paper, geom.NewSize(620, 275), true)
	e.codeView.install(e)
	work.AddChild(e.sourceScroll)

	bar := surface(sage, 2, geom.NewSymmetricInsets(14, 5))
	bar.SetLayoutData(fill(false))
	title := label("Output", unison.EmphasizedSystemFont.Face().Font(15), ink)
	title.SetLayoutData(fill(false))
	bar.AddChild(title)
	bar.AddChild(toolButton("Clear", nil, e.clearOutput))
	work.AddChild(bar)
	e.output.Accessibility.LabeledBy = title
	work.AddChild(fieldScroll(e.output, sage, geom.NewSize(620, 145), false))
	e.input.SetLayoutData(fill(false))
	return work
}

func fieldScroll(f *unison.Field, color unison.Color, size geom.Size, expand bool) *unison.ScrollPanel {
	f.AutoScroll = false
	f.EditableInk, f.OnEditableInk = color, ink
	f.SelectionInk, f.OnSelectionInk = focus, unison.White
	padding := geom.NewSymmetricInsets(12, 8)
	if expand {
		padding.Left = 49
		padding.Top = 12
	}
	installEditorBorder(f, padding)
	s := unison.NewScrollPanel()
	s.BackgroundInk = color
	data := fill(expand)
	data.SizeHint = size
	data.MinSize = geom.NewSize(200, 65)
	s.SetLayoutData(data)
	width := behavior.Follow
	if !f.Wrap() {
		width = behavior.Fill
	}
	s.SetContent(f, width, behavior.HintedFill)
	return s
}

func installEditorBorder(field *unison.Field, padding geom.Insets) {
	space := unison.NewEmptyBorder(padding)
	ring := unison.NewLineBorder(focus, geom.Size{}, geom.NewUniformInsets(1), true)
	focused := unison.NewCompoundBorder(ring, space)
	unison.InstallFocusBorders(field, field, focused, space)
	if field.Focused() {
		field.SetBorder(focused)
	}
}

var (
	folderIcon = unison.MustSVGFromContentString(`<svg viewBox="0 0 24 24"><path d="M2 4h7l3 3h8v2H5l-3 8z M6 11h17l-4 9H2z"/></svg>`)
	saveIcon   = unison.MustSVGFromContentString(`<svg viewBox="0 0 24 24"><path fill-rule="evenodd" d="M2 2h15l5 5v15H2z M6 4v6h10V4z M6 14v6h12v-6z"/></svg>`)
	playIcon   = unison.MustSVGFromContentString(`<svg viewBox="0 0 24 24"><path d="M5 3l16 9-16 9z"/></svg>`)
	stopIcon   = unison.MustSVGFromContentString(`<svg viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="2"/></svg>`)
)
