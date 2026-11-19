// Package book serves the offline Dinoscript workbook and its editable lesson data.
package book

import (
	"embed"
	"encoding/json"

	"github.com/fclaude/dinoscript/internal/locale"
)

const PDFFilename = "Dinoscript-with-Pip.pdf"

//go:embed book*.json Dinoscript-*.pdf art/*.png
var files embed.FS

type Guide struct {
	Title    string
	Subtitle string
	Author   string
	CoverArt string `json:"cover_art"`
	CoverAlt string `json:"cover_alt"`
	Welcome  string
	Pages    []Lesson
}

type Lesson struct {
	Chapter   string
	Title     string
	Pip       string
	Body      []string
	Code      string
	Challenge string
	Hint      string
	Art       string
	Alt       string
	Checks    []Example
}

type Example struct {
	Input  []string
	Output string
}

var editions = map[locale.Language]struct{ source, pdf string }{
	locale.English: {"book.json", PDFFilename},
	locale.Spanish: {"book.es-CL.json", "Dinoscript-con-Pip.pdf"},
	locale.Polish:  {"book.pl.json", "Dinoscript-z-Pip.pdf"},
}

func edition(language locale.Language) (source, pdf string) {
	if entry, ok := editions[language]; ok {
		return entry.source, entry.pdf
	}
	return "book.json", PDFFilename
}

func Filename(language locale.Language) string {
	_, pdf := edition(language)
	return pdf
}

func Load() (Guide, error) { return LoadLanguage(locale.English) }

func LoadLanguage(language locale.Language) (Guide, error) {
	source, _ := edition(language)
	data, err := files.ReadFile(source)
	if err != nil {
		return Guide{}, err
	}
	var guide Guide
	err = json.Unmarshal(data, &guide)
	return guide, err
}

func PDF() ([]byte, error) { return files.ReadFile(PDFFilename) }

func PDFLanguage(language locale.Language) ([]byte, error) { return files.ReadFile(Filename(language)) }

func Art(name string) ([]byte, error) { return files.ReadFile("art/" + name) }
