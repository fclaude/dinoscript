package desktop

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/richardwilkes/unison"
)

type runEvent struct {
	text      string
	readInput bool
}

type execution struct {
	source string
	events chan runEvent
	input  chan string
	cancel context.CancelFunc
	err    error // read after events has been closed
}

func (e *editor) startRun() {
	if e.active != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &execution{
		source: e.source.Text(),
		events: make(chan runEvent, 128),
		input:  make(chan string, 1),
		cancel: cancel,
	}
	e.active = run
	e.diagnostic = nil
	e.outputText.Reset()
	e.output.ApplyFieldState(&unison.FieldState{})
	e.run.SetEnabled(false)
	e.stop.SetEnabled(true)
	e.jump.SetEnabled(false)
	e.setStatus("Running…")
	go run.execute(ctx)
	e.poll(run)
}

func (r *execution) execute(ctx context.Context) {
	defer close(r.events)
	emit := func(event runEvent) bool {
		select {
		case r.events <- event:
			return true
		case <-ctx.Done():
			return false
		}
	}
	r.err = dino.Run(ctx, r.source, dino.Console{
		Output: func(line string) { emit(runEvent{text: line}) },
		Input: func(ctx context.Context) (string, error) {
			if !emit(runEvent{readInput: true}) {
				return "", ctx.Err()
			}
			select {
			case answer := <-r.input:
				return answer, nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	})
}

func (e *editor) poll(run *execution) {
	if e.closed || e.active != run {
		return
	}
	var batch strings.Builder
	for range cap(run.events) {
		select {
		case event, ok := <-run.events:
			if !ok {
				e.appendOutput(batch.String())
				e.finishRun(run)
				return
			}
			batch.WriteString(event.text)
			if event.readInput && e.stop.Enabled() {
				e.setInputEnabled(true)
				e.setStatus("Enter a whole number in Output and press Enter.")
			}
		default:
			e.appendOutput(batch.String())
			unison.InvokeTaskAfter(func() { e.poll(run) }, 25*time.Millisecond)
			return
		}
	}
	e.appendOutput(batch.String())
	unison.InvokeTaskAfter(func() { e.poll(run) }, 25*time.Millisecond)
}

func (e *editor) appendOutput(text string) {
	if text != "" {
		e.outputText.WriteString(text)
		e.output.ApplyFieldState(&unison.FieldState{Text: e.outputText.String()})
		e.output.SetSelectionToEnd()
		e.output.MarkForLayoutAndRedraw()
	}
}

func (e *editor) stopRun() {
	if e.active == nil || !e.stop.Enabled() {
		return
	}
	e.active.cancel()
	e.stop.SetEnabled(false)
	e.setInputEnabled(false)
	e.setStatus("Stopping…")
}

func (e *editor) finishRun(run *execution) {
	run.cancel()
	e.active = nil
	e.run.SetEnabled(true)
	e.stop.SetEnabled(false)
	e.setInputEnabled(false)
	switch {
	case run.err == nil:
		e.setStatus("Finished.")
	case errors.Is(run.err, context.Canceled):
		e.setStatus("Stopped.")
	default:
		e.appendOutput(run.err.Error() + "\n")
		if errors.As(run.err, &e.diagnostic) {
			e.diagnosticSource = run.source
			e.updateJump()
		}
		e.setStatus("Fix the error in Output and run again.")
		unison.AnnounceForAccessibility(run.err.Error())
	}
}

func (e *editor) setInputEnabled(enabled bool) {
	wasEnabled := e.input.Enabled()
	e.input.SetEnabled(enabled)
	e.input.SetText("")
	if enabled {
		e.window.SetFocus(e.input)
	} else if wasEnabled {
		e.window.SetFocus(e.source)
	}
}

func (e *editor) submitInput() {
	if e.active == nil || !e.input.Enabled() {
		return
	}
	text := e.input.Text()
	e.active.input <- text
	e.setInputEnabled(false)
	if e.outputText.Len() < 64*1024 {
		e.appendOutput("> " + text + "\n")
	}
	e.setStatus("Running…")
}

func (e *editor) updateJump() {
	e.jump.SetEnabled(e.diagnostic != nil && e.source.Text() == e.diagnosticSource)
}

func (e *editor) jumpToError() {
	if !e.jump.Enabled() {
		return
	}
	span := e.diagnostic.Span
	source := e.diagnosticSource
	e.window.SetFocus(e.source)
	e.source.SetSelection(utf8.RuneCountInString(source[:span.Start]), utf8.RuneCountInString(source[:span.End]))
}
