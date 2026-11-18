package desktop

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fclaude/dinoscript/book"
	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/fclaude/dinoscript/internal/locale"
)

func TestLessons(t *testing.T) {
	for _, tt := range []struct {
		name, file, input, want string
	}{
		{"first run", "hello.ds", "", "Hello, dinosaur!\n"},
		{"decision", "decisions.ds", "", "You can ride the big dinosaur.\n"},
		{"loop", "counting.ds", "", "Dinosaur 1\nDinosaur 2\nDinosaur 3\nDinosaur 4\nDinosaur 5\n"},
		{"fibonacci", "fibonacci.ds", "", "Fibonacci number 8 is 21\n"},
		{"prime one", "prime.ds", "1", "1 is not prime.\n"},
		{"prime two", "prime.ds", "2", "2 is prime.\n"},
		{"prime nine", "prime.ds", "9", "9 is not prime.\n"},
		{"prime twenty-five", "prime.ds", "25", "25 is not prime.\n"},
		{"prime twenty-nine", "prime.ds", "29", "29 is prime.\n"},
		{"hola", "es-CL/hola.ds", "", "¡Hola, dinosaurio!\n"},
		{"decisiones", "es-CL/decisiones.ds", "", "Puedes subir al dinosaurio grande.\n"},
		{"contar", "es-CL/contar.ds", "", "Dinosaurio 1\nDinosaurio 2\nDinosaurio 3\nDinosaurio 4\nDinosaurio 5\n"},
		{"fibonacci en español", "es-CL/serie-fibonacci.ds", "", "El número de Fibonacci en la posición 8 es 21\n"},
		{"primo", "es-CL/primo.ds", "29", "¿Qué número entero quieres revisar?\n29 es primo.\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			err := dino.Run(context.Background(), example(tt.file), dino.Console{
				Input:  func(context.Context) (string, error) { return tt.input, nil },
				Output: func(s string) { output.WriteString(s) },
			})
			want := tt.want
			if tt.input != "" && tt.file == "prime.ds" {
				want = "Which whole number do you want to check?\n" + want
			}
			if err != nil || output.String() != want {
				t.Fatalf("lesson output = %q, error = %v; want %q", output.String(), err, want)
			}
		})
	}
}

func TestNumberMagicWorkbookCode(t *testing.T) {
	for _, language := range locale.Languages() {
		t.Run(string(language), func(t *testing.T) {
			guide, err := book.LoadLanguage(language)
			if err != nil {
				t.Fatal(err)
			}
			app := editor{language: language}
			_, source := app.sample(numberMagicExample)
			if !slices.ContainsFunc(guide.Pages, func(lesson book.Lesson) bool { return lesson.Code == source }) {
				t.Fatal("the bundled example and workbook code differ")
			}
		})
	}
}
