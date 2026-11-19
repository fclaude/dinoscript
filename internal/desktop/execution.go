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

type inputKind uint8

const (
	numberInput inputKind = iota + 1
	continueInput
)

type runEvent struct {
	text  string
	input inputKind
}

type execution struct {
	source string
	events chan runEvent
	input  chan string
	cancel context.CancelFunc
	err    error // read after events has been closed

	inputKind inputKind // used on the UI thread
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
	e.clearOutput()
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
	readInput := func(ctx context.Context, kind inputKind) (string, error) {
		if !emit(runEvent{input: kind}) {
			return "", ctx.Err()
		}
		select {
		case answer := <-r.input:
			return answer, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	r.err = dino.Run(ctx, r.source, dino.Console{
		Output: func(line string) { emit(runEvent{text: line}) },
		Input:  func(ctx context.Context) (string, error) { return readInput(ctx, numberInput) },
		Pause: func(ctx context.Context) error {
			_, err := readInput(ctx, continueInput)
			return err
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
			if event.input != 0 && e.stop.Enabled() {
				run.inputKind = event.input
				e.setInputEnabled(true)
				if event.input == continueInput {
					e.setStatus("Press Enter to continue.")
				} else {
					e.setStatus("Enter a whole number in Output and press Enter.")
				}
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
		e.renderOutput()
	}
}

func (e *editor) renderOutput() {
	text := e.outputText.String()
	if e.diagnostic != nil {
		text += e.diagnostic.Localized(e.language) + "\n"
	}
	bar := e.outputScroll.Bar(false)
	follow := bar.Value() == bar.MaxValue()
	state := e.output.GetFieldState()
	state.Text = text
	e.output.ApplyFieldState(state)
	e.outputScroll.MarkForLayoutAndRedraw()
	if follow {
		e.window.ValidateLayout()
		e.outputScroll.SetPosition(0, bar.MaxValue())
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
		message := run.err.Error()
		if errors.As(run.err, &e.diagnostic) {
			message = e.diagnostic.Localized(e.language)
			e.diagnosticSource = run.source
			e.renderOutput()
			e.updateJump()
		} else {
			e.appendOutput(message + "\n")
		}
		e.setStatus("Fix the error in Output and run again.")
		unison.AnnounceForAccessibility(message)
	}
}

func (e *editor) setInputEnabled(enabled bool) {
	wasEnabled := e.input.Enabled()
	if enabled && e.input.Parent() == nil {
		e.sourceScroll.Parent().AddChild(e.input)
	} else if !enabled && e.input.Parent() != nil {
		e.input.RemoveFromParent()
	}
	e.input.SetEnabled(enabled)
	e.input.MarkForLayoutAndRedraw()
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
	if e.active.inputKind == numberInput && e.outputText.Len() < 64*1024 {
		e.appendOutput("> " + text + "\n")
	}
	e.setStatus("Running…")
}

func (e *editor) updateJump() {
	enabled := e.diagnostic != nil && e.source.Text() == e.diagnosticSource
	if enabled && e.jump.Parent() == nil {
		e.status.Parent().AddChildAtIndex(e.jump, 1)
	} else if !enabled && e.jump.Parent() != nil {
		e.jump.RemoveFromParent()
	}
	e.jump.SetEnabled(enabled)
	e.jump.MarkForLayoutAndRedraw()
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
