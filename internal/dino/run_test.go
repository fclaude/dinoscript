package dino

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunPrograms(t *testing.T) {
	for _, tt := range []struct {
		name, source, want string
	}{
		{"blank and comment", "# nothing to run\r\n\r", ""},
		{"arithmetic and precedence", "print(2 + 3 * 4, \" \", 2^3^2, \" \", -2^2, \" \", 2^-3)", "14 512 -4 0.125\n"},
		{"division and decimals", "print(-7 // 3, \" \", -7 % 3, \" \", 7 / 2, \" \", 1.5e2 + 0.5)", "-2 -1 3.5 150.5\n"},
		{"whole numbers stay exact", "print(-9223372036854775808, \" \", 9007199254740993 = 9007199254740992.0)", "-9223372036854775808 false\n"},
		{"logical precedence", "print(not 2 = 3 and true or false, \" \", 1 < 2 and 3 >= 3)", "true true\n"},
		{"short circuit", "print(true or missing, \" \", false and readint())", "true false\n"},
		{"quoted text and equality", "print(\"🦖\", \"\\n\", \"a\" != \"b\", \" \", false = false)", "🦖\ntrue true\n"},
		{"bare CR and CRLF", "let i = 2\rif i < 3 then\r\n  print(i)\r\nend if", "2\n"},
		{"nested return and recursion", lines(
			"let count = 1",
			"function fib(n)",
			"  if n <= count then",
			"    output n",
			"  else",
			"    output fib(n-1) + fib(n-2)",
			"  end if",
			"end function",
			"function countTo(n)",
			"  let count = count + 1",
			"  while count <= n do",
			"    if count = n then",
			"      output count",
			"    end if",
			"    let count = count + 1",
			"  end while",
			"  output 0",
			"end function",
			"print(fib(8), \" \", countTo(4), \" \", count)",
		), "21 4 1\n"},
		{"function without result", lines("function sayHi()", "  print(\"Hi\")", "end function", "sayHi()"), "Hi\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collect(context.Background(), tt.source, nil)
			if err != nil || got != tt.want {
				t.Fatalf("output = %q, error = %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name, source, message string
		line, column          int
	}{
		{"missing end if", lines("print(1)", "if true then", "  print(2)"), "Add \"end if\"", 2, 1},
		{"wrong block closer", lines("while true do", "end if"), "Add \"while\"", 2, 5},
		{"unicode span", "print(\"🦖\", missing)", "I don't know \"missing\"", 1, 12},
		{"function arity", lines("function foo(x)", "  output x", "end function", "foo()"), "expects 1 argument", 4, 1},
		{"void result", "let x = print(1)", "didn't give back a value", 1, 9},
		{"top-level output", "output 1", "Use output inside a function", 1, 1},
		{"division by zero", "print(5 / 0)", "divide by zero", 1, 9},
		{"integer overflow", "print(9223372036854775807 + 1)", "too big", 1, 27},
		{"integer division overflow", "print(-9223372036854775808 // -1)", "too big", 1, 28},
		{"condition type", lines("if 1 then", "end if"), "needs true or false", 1, 4},
		{"empty argument", "print(,)", "Add a value", 1, 7},
		{"malformed string", "print(\"hello", "closing double quote", 1, 7},
		{"invalid exponent", "print(1e)", "valid decimal number", 1, 7},
		{"missing input", "readint()", "there is no input", 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := collect(context.Background(), tt.source, nil)
			var diagnostic *Diagnostic
			if !errors.As(err, &diagnostic) {
				t.Fatalf("expected a source diagnostic, got %v", err)
			}
			if !strings.Contains(diagnostic.Message, tt.message) || diagnostic.Span.Line != tt.line || diagnostic.Span.Column != tt.column {
				t.Fatalf("diagnostic = %v, want %q at %d:%d", diagnostic, tt.message, tt.line, tt.column)
			}
			if tt.name == "unicode span" && tt.source[diagnostic.Span.Start:diagnostic.Span.End] != "missing" {
				t.Fatalf("byte range %+v does not select missing", diagnostic.Span)
			}
		})
	}
}

func TestRunInput(t *testing.T) {
	source := lines("print(\"A number?\")", "let n = readint()", "print(n + readint())")
	replies := []string{" 12 ", "-5"}
	output, err := collect(context.Background(), source, func(context.Context) (string, error) {
		if len(replies) == 0 {
			return "", io.EOF
		}
		line := replies[0]
		replies = replies[1:]
		return line, nil
	})
	if err != nil || output != "A number?\n7\n" {
		t.Fatalf("output = %q, error = %v", output, err)
	}
	_, err = collect(context.Background(), "readint()", func(context.Context) (string, error) { return "five", nil })
	if err == nil || !strings.Contains(err.Error(), "isn't a whole number") {
		t.Fatalf("invalid input returned %v", err)
	}
}

func TestLimitsAndFreshRun(t *testing.T) {
	for _, tt := range []struct{ name, source, want string }{
		{"steps", lines("while true do", "end while"), "too many steps"},
		{"recursion", lines("function again()", "  again()", "end function", "again()"), "too many unfinished"},
		{"output", lines("while true do", "  print(\""+strings.Repeat("x", 500)+"\")", "end while"), "output is full"},
		{"nesting", strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200), "nested too deeply"},
		{"arguments", "print(" + strings.Repeat("1,", maxArguments) + "1)", "too many arguments"},
		{"source size", strings.Repeat(" ", MaxSourceBytes+1), "too large"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := collect(context.Background(), tt.source, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			got, err := collect(context.Background(), "print(7)", nil)
			if err != nil || got != "7\n" {
				t.Fatalf("following run output = %q, error = %v", got, err)
			}
		})
	}
	_, err := collect(context.Background(), "function secret()\nend function", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = collect(context.Background(), "secret()", nil)
	if err == nil || !strings.Contains(err.Error(), "can't find") {
		t.Fatalf("a function survived across runs: %v", err)
	}
}

func TestCancelDuringInput(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	waiting := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := collect(ctx, "readint()", func(ctx context.Context) (string, error) {
			close(waiting)
			<-ctx.Done()
			return "", ctx.Err()
		})
		done <- err
	}()
	select {
	case <-waiting:
		stop()
	case <-ctx.Done():
		t.Fatal("program never requested input")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("after Stop, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("program did not stop")
	}
}

func FuzzRun(f *testing.F) {
	f.Add("print(2+2)")
	f.Add("function f(x)\n  output f(x - 1)\nend function\nf(1)")
	f.Add("if 1 then\nelse\nend while")
	f.Add("print(\"🦖\\t\", -9223372036854775808)")
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			return
		}
		_, _ = collect(context.Background(), source, nil)
	})
}

func collect(ctx context.Context, source string, input func(context.Context) (string, error)) (string, error) {
	var output strings.Builder
	err := Run(ctx, source, Console{Output: func(line string) { output.WriteString(line) }, Input: input})
	return output.String(), err
}

func lines(items ...string) string { return strings.Join(items, "\n") }
