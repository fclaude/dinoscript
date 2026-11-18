package desktop

import (
	"context"
	"strings"
	"testing"

	"github.com/fclaude/dinoscript/internal/dino"
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			err := dino.Run(context.Background(), example(tt.file), dino.Console{
				Input:  func(context.Context) (string, error) { return tt.input, nil },
				Output: func(s string) { output.WriteString(s) },
			})
			want := tt.want
			if tt.input != "" {
				want = "Which whole number do you want to check?\n" + want
			}
			if err != nil || output.String() != want {
				t.Fatalf("lesson output = %q, error = %v; want %q", output.String(), err, want)
			}
		})
	}
}
