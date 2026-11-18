// Dinoscript-web connects the interpreter to a dedicated browser worker.
package main

import (
	"context"
	"errors"
	"strings"
	"syscall/js"
	"time"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/fclaude/dinoscript/internal/locale"
)

type execution struct {
	answer  chan string
	waiting bool
	output  strings.Builder
	sentAt  time.Time
}

func post(message map[string]any) {
	js.Global().Call("postMessage", message)
}

func (r *execution) flush() {
	if r.output.Len() > 0 {
		post(map[string]any{"kind": "output", "text": r.output.String()})
		r.output.Reset()
		r.sentAt = time.Now()
	}
}

func (r *execution) read(kind string) string {
	r.flush()
	r.waiting = true
	post(map[string]any{"kind": kind})
	return <-r.answer
}

func (r *execution) run(source string, language locale.Language) error {
	err := dino.Run(context.Background(), source, dino.Console{
		Output: func(line string) {
			r.output.WriteString(line)
			if r.output.Len() >= 1024 || time.Since(r.sentAt) >= 30*time.Millisecond {
				r.flush()
			}
		},
		Input: func(context.Context) (string, error) { return r.read("number"), nil },
		Pause: func(context.Context) error { r.read("pause"); return nil },
	})
	r.flush()
	return err
}

func main() {
	var active *execution
	handler := js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := args[0].Get("data")
		switch message.Get("kind").String() {
		case "run":
			if active != nil {
				return nil
			}
			source := message.Get("source").String()
			language := locale.Language(message.Get("language").String())
			run := &execution{answer: make(chan string, 1)}
			active = run
			go func() {
				err := run.run(source, language)
				active = nil
				var diagnostic *dino.Diagnostic
				switch {
				case errors.As(err, &diagnostic):
					post(map[string]any{
						"kind": "diagnostic", "text": diagnostic.Localized(language),
						"start": diagnostic.Span.Start, "end": diagnostic.Span.End, "line": diagnostic.Span.Line,
					})
				case err != nil:
					post(map[string]any{"kind": "error"})
				default:
					post(map[string]any{"kind": "done"})
				}
			}()
		case "answer":
			if active != nil && active.waiting {
				active.waiting = false
				active.answer <- message.Get("text").String()
			}
		}
		return nil
	})
	js.Global().Set("onmessage", handler)
	post(map[string]any{"kind": "ready"})
	select {}
}
