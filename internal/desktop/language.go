package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fclaude/dinoscript/internal/locale"
	"github.com/richardwilkes/unison"
)

const systemLanguage locale.Language = "system"

var uiMessages = map[locale.Language]map[string]string{
	locale.Spanish: spanishUI,
	locale.Polish:  polishUI,
}

// A status argument which must be translated with the surrounding message.
type statusMessage string

func (e *editor) text(message string, args ...any) string {
	if translated, ok := uiMessages[e.language][message]; ok {
		message = translated
	}
	if len(args) != 0 {
		return fmt.Sprintf(message, args...)
	}
	return message
}

func (e *editor) languageMenu() *unison.PopupMenu[locale.Language] {
	menu := toolbarMenu[locale.Language]()
	menu.AddItem(systemLanguage)
	menu.AddItem(locale.Languages()...)
	menu.ItemRendererCallback = func(choice locale.Language) string {
		if choice == systemLanguage {
			return e.text("System language")
		}
		return choice.Name()
	}
	e.localize(func() {
		menu.Accessibility.Name = e.text("Language")
		menu.Tooltip = unison.NewTooltipWithText(e.text("Language"))
		menu.MarkForLayoutAndRedraw()
	})
	menu.Select(e.preference)
	menu.ChoiceMadeCallback = func(_ *unison.PopupMenu[locale.Language], _ int, choice locale.Language) {
		e.changeLanguage(choice)
		menu.Select(choice)
	}
	return menu
}

func (e *editor) localize(update func()) {
	e.localized = append(e.localized, update)
	update()
}

func (e *editor) localizeTitle(control interface {
	SetTitle(string)
	MarkForLayoutAndRedraw()
}, message string) {
	e.localize(func() {
		control.SetTitle(e.text(message))
		control.MarkForLayoutAndRedraw()
	})
}

func (e *editor) useLanguage(preference locale.Language) {
	e.preference = preference
	e.language = preference
	if preference == systemLanguage {
		e.language = e.systemLocale
	}
}

func (e *editor) changeLanguage(preference locale.Language) {
	if preference == e.preference {
		return
	}
	previous := e.language
	e.useLanguage(preference)
	if previous != e.language {
		for _, update := range e.localized {
			update()
		}
		for _, tab := range e.tabs {
			if tab != e.document {
				e.styleTab(tab)
			}
		}
		e.updateTabTitle()
		e.refreshPosition()
		e.renderStatus()
		if e.diagnostic != nil {
			e.renderOutput()
		}
		if e.reader != nil {
			e.reader.refreshLanguage()
		}
	}
	if err := e.saveLanguage(); err != nil {
		e.setStatus("Couldn't save the language preference.")
	}
}

func (e *editor) languagePath() string {
	return filepath.Join(filepath.Dir(e.draftPath), "language")
}

func (e *editor) loadLanguage() {
	e.useLanguage(systemLanguage)
	if e.draftPath == "" {
		return
	}
	data, err := os.ReadFile(e.languagePath())
	if err == nil {
		preference := locale.Language(strings.TrimSpace(string(data)))
		if preference.Name() != "" {
			e.useLanguage(preference)
		}
	}
}

func (e *editor) saveLanguage() error {
	if e.draftPath == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(e.draftPath), 0700); err != nil {
		return err
	}
	return writeFile(e.languagePath(), []byte(string(e.preference)+"\n"))
}
