package dino

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSourceBytes bounds source input for the interpreter and its callers.
const MaxSourceBytes = 64 * 1024

// MaxInputBytes bounds numeric input.
const MaxInputBytes = 1024

const (
	maxOutputBytes = 64 * 1024
	maxSteps       = 300_000
	maxCalls       = 96
	maxEvalDepth   = 512
	executionTime  = 2 * time.Second
)

// Console connects a program to its host. Input waits for one line. Pause is
// optional and lets interactive hosts handle Enter separately. If Pause is nil,
// a pause consumes a line from Input. Both callbacks must respect cancellation.
type Console struct {
	Input  func(context.Context) (string, error)
	Pause  func(context.Context) error
	Output func(string)
}

// Run parses and executes one program in a fresh environment.
func Run(ctx context.Context, source string, console Console) error {
	first := Span{Line: 1, Column: 1}
	if len(source) > MaxSourceBytes {
		return problem(first, "This program is too large. Try a smaller file.")
	}
	if !utf8.ValidString(source) {
		return problem(first, "Open a source file saved as UTF-8 text.")
	}
	r := runner{
		ctx: ctx, console: console, deadline: time.Now().Add(executionTime),
		globals: make(map[string]value), functions: make(map[string]statement),
	}
	parseCtx, cancel := context.WithDeadline(ctx, r.deadline)
	program, err := parse(parseCtx, source)
	cancel()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return timeLimit(first)
		}
		return err
	}
	_, err = r.block(program, r.globals)
	return err
}

type runner struct {
	ctx       context.Context
	console   Console
	globals   map[string]value
	functions map[string]statement
	deadline  time.Time
	steps     int
	calls     int
	evalDepth int
	bytesOut  int
}

func (r *runner) step(at Span) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if time.Now().After(r.deadline) {
		return timeLimit(at)
	}
	r.steps++
	if r.steps > maxSteps {
		return problem(at, "This program took too many steps. Check whether your loop can finish.")
	}
	return nil
}

func timeLimit(at Span) error {
	return problem(at, "This program ran out of time. Try fewer steps.")
}

// A nonempty result propagates output statements back to the current call.
func (r *runner) block(body []statement, locals map[string]value) (value, error) {
	for _, s := range body {
		if err := r.step(s.start.span); err != nil {
			return value{}, err
		}
		result, err := r.statement(s, locals)
		if err != nil || result.kind != noValue {
			return result, err
		}
	}
	return value{}, nil
}

func (r *runner) statement(s statement, locals map[string]value) (value, error) {
	switch s.start.text {
	case "let":
		result, err := r.requireValue(s.value, locals)
		if err != nil {
			return value{}, err
		}
		locals[s.name.text] = result
	case "output":
		return r.requireValue(s.value, locals)
	case "if":
		yes, err := r.condition(s.value, locals)
		if err != nil {
			return value{}, err
		}
		if yes {
			return r.block(s.body, locals)
		}
		return r.block(s.alternate, locals)
	case "while":
		for {
			yes, err := r.condition(s.value, locals)
			if err != nil || !yes {
				return value{}, err
			}
			result, err := r.block(s.body, locals)
			if err != nil || result.kind != noValue {
				return result, err
			}
		}
	case "function":
		if _, exists := r.functions[s.name.text]; exists || s.name.text == "print" || s.name.text == "readint" {
			return value{}, problem(s.name.span, "Choose a different name. %q is already a function.", s.name.text)
		}
		r.functions[s.name.text] = s
	default:
		if _, err := r.eval(s.value, locals); err != nil {
			return value{}, err
		}
	}
	return value{}, nil
}

func (r *runner) condition(expr *expression, locals map[string]value) (bool, error) {
	result, err := r.requireValue(expr, locals)
	if err != nil {
		return false, err
	}
	if result.kind != boolValue {
		return false, problem(expr.at.span, "This condition needs true or false. Try a comparison.")
	}
	return result.flag, nil
}

func (r *runner) requireValue(expr *expression, locals map[string]value) (value, error) {
	result, err := r.eval(expr, locals)
	if err == nil && result.kind == noValue {
		return value{}, problem(expr.at.span, "This call didn't give back a value. Use an output statement in the function.")
	}
	return result, err
}

func (r *runner) eval(expr *expression, locals map[string]value) (value, error) {
	if err := r.step(expr.at.span); err != nil {
		return value{}, err
	}
	if r.evalDepth == maxEvalDepth {
		return value{}, problem(expr.at.span, "This expression is nested too deeply. Try using smaller expressions.")
	}
	r.evalDepth++
	defer func() { r.evalDepth-- }()
	switch {
	case expr.literal.kind != noValue:
		return expr.literal, nil
	case expr.call:
		return r.call(expr, locals)
	case expr.left != nil:
		return r.binary(expr, locals)
	case expr.right != nil:
		right, err := r.requireValue(expr.right, locals)
		if err != nil {
			return value{}, err
		}
		return unary(expr.at, right)
	default:
		if found, ok := locals[expr.at.text]; ok {
			return found, nil
		}
		if found, ok := r.globals[expr.at.text]; ok {
			return found, nil
		}
		return value{}, problem(expr.at.span, "I don't know %q yet. Give it a value with let before using it.", expr.at.text)
	}
}

func (r *runner) call(expr *expression, locals map[string]value) (value, error) {
	name := expr.at.text
	if name == "readint" {
		if len(expr.args) != 0 {
			return value{}, problem(expr.at.span, "Use readint() with nothing inside the parentheses.")
		}
		return r.readInt(expr.at.span)
	}
	function, found := r.functions[name]
	if name == "pause" && !found {
		if len(expr.args) != 0 {
			return value{}, problem(expr.at.span, "Use pause() with nothing inside the parentheses.")
		}
		return value{}, r.pause(expr.at.span)
	}
	if name != "print" {
		if !found {
			return value{}, problem(expr.at.span, "I can't find the function %q. Define it above this call.", name)
		}
		if len(expr.args) != len(function.parameters) {
			return value{}, problem(expr.at.span, "%s expects %d argument(s). This call gives it %d.", name, len(function.parameters), len(expr.args))
		}
	}
	arguments := make([]value, 0, len(expr.args))
	for _, arg := range expr.args {
		result, err := r.requireValue(arg, locals)
		if err != nil {
			return value{}, err
		}
		arguments = append(arguments, result)
	}
	if name == "print" {
		return value{}, r.print(expr.at.span, arguments)
	}
	if r.calls == maxCalls {
		return value{}, problem(expr.at.span, "There are too many unfinished function calls. Check when the function stops calling itself.")
	}
	r.calls++
	defer func() { r.calls-- }()
	frame := make(map[string]value, len(arguments))
	for i, result := range arguments {
		frame[function.parameters[i].text] = result
	}
	return r.block(function.body, frame)
}

func (r *runner) print(at Span, arguments []value) error {
	var line strings.Builder
	for _, arg := range arguments {
		part := arg.String()
		if r.bytesOut+line.Len()+len(part)+1 > maxOutputBytes {
			return problem(at, "The output is full. Try printing fewer lines or shorter text.")
		}
		line.WriteString(part)
	}
	if r.bytesOut+line.Len()+1 > maxOutputBytes {
		return problem(at, "The output is full. Try printing fewer lines.")
	}
	line.WriteByte('\n')
	r.bytesOut += line.Len()
	if r.console.Output != nil {
		r.console.Output(line.String())
	}
	return nil
}

func (r *runner) readLine(at Span, input func(context.Context) (string, error), missing, exhausted string) (string, error) {
	if input == nil {
		return "", &Diagnostic{Span: at, Message: missing, format: missing}
	}
	start := time.Now()
	line, err := input(r.ctx)
	r.deadline = r.deadline.Add(time.Since(start))
	if r.ctx.Err() != nil {
		return "", r.ctx.Err()
	}
	if errors.Is(err, io.EOF) {
		return "", &Diagnostic{Span: at, Message: exhausted, format: exhausted}
	}
	if err != nil {
		return "", problem(at, "Couldn't read input. Try running the program again.")
	}
	return line, nil
}

func (r *runner) pause(at Span) error {
	input := r.console.Input
	if r.console.Pause != nil {
		input = func(ctx context.Context) (string, error) { return "", r.console.Pause(ctx) }
	}
	_, err := r.readLine(at, input, "pause() needs input so it can wait for Enter.", "pause() needs another input line to continue.")
	return err
}

func (r *runner) readInt(at Span) (value, error) {
	line, err := r.readLine(at, r.console.Input, "readint() needs a whole number, but there is no input.", "readint() needs another line with a whole number.")
	if err != nil {
		return value{}, err
	}
	if len(line) > MaxInputBytes {
		return value{}, problem(at, "That answer is too long. Enter one whole number.")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	if err != nil {
		return value{}, problem(at, "That answer isn't a whole number. Run again and enter a number such as 7.")
	}
	return integer(n), nil
}
