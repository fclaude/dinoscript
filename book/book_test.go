package book

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/fclaude/dinoscript/internal/locale"
)

func TestLessonPrograms(t *testing.T) {
	for _, language := range locale.Languages() {
		t.Run(string(language), func(t *testing.T) { checkLessonPrograms(t, language) })
	}
}

func checkLessonPrograms(t *testing.T, language locale.Language) {
	guide, err := LoadLanguage(language)
	if err != nil {
		t.Fatal(err)
	}
	if guide.Author != "Francisco Claude-Faust" || len(guide.Pages) != 20 {
		t.Fatalf("unexpected author or lesson count: %q, %d", guide.Author, len(guide.Pages))
	}
	for _, lesson := range guide.Pages {
		t.Run(lesson.Title, func(t *testing.T) {
			if len(lesson.Checks) == 0 {
				t.Fatal("missing expected output")
			}
			for _, example := range lesson.Checks {
				var output strings.Builder
				answers := example.Input
				err := dino.Run(context.Background(), lesson.Code, dino.Console{
					Output: func(line string) { output.WriteString(line) },
					Input: func(context.Context) (string, error) {
						if len(answers) == 0 {
							return "", io.EOF
						}
						answer := answers[0]
						answers = answers[1:]
						return answer, nil
					},
				})
				if err != nil || output.String() != example.Output || len(answers) > 0 {
					t.Errorf("inputs %v: output %q, error %v; want %q", example.Input, output.String(), err, example.Output)
				}
			}
		})
	}
}

func TestPDFMatchesLessonSourceAndIllustrations(t *testing.T) {
	for _, language := range locale.Languages() {
		t.Run(string(language), func(t *testing.T) { checkPDFSource(t, language) })
	}
}

func checkPDFSource(t *testing.T, language locale.Language) {
	source, pdfName := edition(language)
	data, err := files.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.New()
	digest.Write(data)
	art, err := files.ReadDir("art")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, image := range art {
		names = append(names, image.Name())
		data, err := Art(image.Name())
		if err != nil {
			t.Fatal(err)
		}
		digest.Write(data)
	}
	fonts, err := os.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	for _, font := range fonts {
		if filepath.Ext(font.Name()) != ".ttf" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("fonts", font.Name()))
		if err != nil {
			t.Fatal(err)
		}
		digest.Write(data)
	}

	guide, err := LoadLanguage(language)
	if err != nil {
		t.Fatal(err)
	}
	if guide.CoverAlt == "" || !slices.Contains(names, guide.CoverArt) {
		t.Fatal("cover must have a bundled image and alt text")
	}
	for _, lesson := range guide.Pages {
		if lesson.Art != "" && (lesson.Alt == "" || !slices.Contains(names, lesson.Art)) {
			t.Errorf("%s must have a bundled image and alt text", lesson.Title)
		}
	}
	pdf, err := files.ReadFile(pdfName)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) ||
		!bytes.Contains(pdf, []byte("source-sha256 "+hex.EncodeToString(digest.Sum(nil)))) {
		t.Fatal("rebuild the PDF with python3 scripts/build-book.py")
	}
}
