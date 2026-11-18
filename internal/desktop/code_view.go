package desktop

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
)

// The field owns editing, hit testing, selection, accessibility and the blinking caret.
// Its decoration uses the same font metrics and leaves the native selection and caret visible.
type codeDecoration struct {
	lines  []codeLine
	font   unison.Font
	source string
	styles map[string]*unison.Text
}

type codeLine struct {
	text      *unison.Text
	start     int // rune index in the field
	y, height float32
}

var codeTokens = regexp.MustCompile(`#.*|"([^"\\]|\\.)*"?|[A-Za-z_][A-Za-z_0-9]*|[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?`)

func tintedLine(source string, font unison.Font) *unison.Text {
	d := unison.TextDecoration{Font: font, OnBackgroundInk: ink}
	text := unison.NewText("", &d)
	pos := 0
	for _, r := range codeTokens.FindAllStringIndex(source, -1) {
		token := source[r[0]:r[1]]
		var color unison.Color
		switch token[0] {
		case '#':
			color = muted
		case '"':
			color = 0xff17705e
		default:
			switch token {
			case "let", "while", "do", "end", "function", "output", "if", "then", "else", "true", "false", "and", "or", "not":
				color = 0xff824477
			case "print", "readint":
				color = 0xff884b2d
			default:
				if token[0] < '0' || token[0] > '9' {
					continue
				}
				color = 0xff99532f
			}
		}
		d.OnBackgroundInk = ink
		text.AddString(source[pos:r[0]], &d)
		d.OnBackgroundInk = color
		text.AddString(token, &d)
		pos = r[1]
	}
	d.OnBackgroundInk = ink
	text.AddString(source[pos:], &d)
	return text
}

func (v *codeDecoration) install(e *editor) {
	e.source.DrawOverCallback = func(c *unison.Canvas, dirty geom.Rect) {
		v.draw(e.source, c, dirty)
		_, pos := e.source.Selection()
		row := sort.Search(len(v.lines), func(i int) bool { return v.lines[i].start > pos }) - 1
		if row >= 0 {
			title := fmt.Sprintf("Ln %d, Col %d", row+1, pos-v.lines[row].start+1)
			if title != e.position.String() {
				e.position.SetTitle(title)
				e.position.MarkForLayoutAndRedraw()
			}
		}
	}
}

func (e *editor) refreshCode() {
	view := &e.codeView
	source := e.source.Text()
	if view.font == e.source.Font && view.source == source {
		return
	}
	previous := view.styles
	if view.font != e.source.Font {
		previous = nil
	}
	styles := make(map[string]*unison.Text)
	var start int
	var y float32
	lines := make([]codeLine, 0, strings.Count(source, "\n")+1)
	for line := range strings.SplitSeq(source, "\n") {
		t := previous[line]
		if t == nil {
			t = styles[line]
			if t == nil {
				t = tintedLine(line, e.source.Font)
			}
		}
		styles[line] = t
		h := max(t.Height(), e.source.Font.LineHeight())
		lines = append(lines, codeLine{text: t, start: start, y: y, height: h})
		y += h
		start += utf8.RuneCountInString(line) + 1
	}
	view.lines, view.styles, view.source, view.font = lines, styles, source, e.source.Font
	padding := e.source.Border().Insets()
	width := max(49, e.source.Font.SimpleWidth(strconv.Itoa(len(lines)))+20)
	if padding.Left != width {
		padding.Left = width
		installEditorBorder(e.source, padding)
	}
}

func (v *codeDecoration) draw(f *unison.Field, c *unison.Canvas, dirty geom.Rect) {
	origin := f.ContentRect(false).Point.Add(f.ScrollOffset())
	origin.X++ // The native field insets left-aligned text by one pixel for its caret.
	start, end := f.Selection()
	first := sort.Search(len(v.lines), func(i int) bool { return origin.Y+v.lines[i].y+v.lines[i].height > dirty.Y })
	for i := first; i < len(v.lines); i++ {
		line := v.lines[i]
		top := origin.Y + line.y
		if top >= dirty.Bottom() {
			break
		}
		base := top + line.text.Baseline()
		number := strconv.Itoa(i + 1)
		c.DrawSimpleString(number, geom.NewPoint(origin.X-12-f.Font.SimpleWidth(number), top+f.Font.Baseline()), f.Font, muted.Paint(c, dirty, paintstyle.Fill))
		if line.text.Empty() {
			continue
		}
		c.Save()
		r := geom.NewRect(origin.X, top, line.text.Width(), line.height)
		if f.Focused() && start <= line.start+len(line.text.Runes()) && end >= line.start {
			left := line.text.PositionForRuneIndex(max(0, start-line.start))
			right := line.text.PositionForRuneIndex(min(end-line.start, len(line.text.Runes())))
			c.ClipRect(geom.NewRect(origin.X+left-1, top, right-left+2, line.height), pathop.Difference, false)
		}
		c.DrawRect(r, paper.Paint(c, r, paintstyle.Fill))
		line.text.Draw(c, geom.NewPoint(origin.X, base))
		c.Restore()
	}
}
