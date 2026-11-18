// Package book serves the offline Dinoscript workbook and its editable lesson data.
package book

import (
	"embed"
	"encoding/json"
)

const PDFFilename = "Dinoscript-with-Pip.pdf"

//go:embed book.json Dinoscript-with-Pip.pdf art/*.png
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

func Load() (Guide, error) {
	data, err := files.ReadFile("book.json")
	if err != nil {
		return Guide{}, err
	}
	var guide Guide
	err = json.Unmarshal(data, &guide)
	return guide, err
}

func PDF() ([]byte, error) { return files.ReadFile(PDFFilename) }

func Art(name string) ([]byte, error) { return files.ReadFile("art/" + name) }
