package dino

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fclaude/dinoscript/internal/locale"
)

// Span identifies a range of UTF-8 bytes and its one-based line and rune column.
type Span struct {
	Start, End   int
	Line, Column int
}

// Diagnostic describes a problem the editor can take the learner to.
type Diagnostic struct {
	Span    Span
	Message string
	format  string
	args    []any
}

var diagnosticMessages = map[locale.Language]map[string]string{
	locale.Spanish: spanishDiagnostics,
	locale.Polish:  polishDiagnostics,
}

func (d *Diagnostic) Error() string {
	return fmt.Sprintf("Line %d, column %d: %s", d.Span.Line, d.Span.Column, d.Message)
}

// Localized formats the diagnostic without changing its source location.
func (d *Diagnostic) Localized(language locale.Language) string {
	if messages := diagnosticMessages[language]; messages != nil {
		if format, ok := messages[d.format]; ok {
			body := fmt.Sprintf(format, d.args...)
			return fmt.Sprintf(messages["Line %d, column %d: %s"], d.Span.Line, d.Span.Column, body)
		}
	}
	return d.Error()
}

func problem(at Span, format string, args ...any) *Diagnostic {
	return &Diagnostic{Span: at, Message: fmt.Sprintf(format, args...), format: format, args: args}
}

type tokenKind uint8

const (
	end tokenKind = iota
	newline
	word
	number
	quoted
	symbol
)

type token struct {
	kind tokenKind
	text string
	span Span
}

type lexer struct {
	source       string
	offset       int
	line, column int
}

func lex(ctx context.Context, source string) ([]token, error) {
	scan := lexer{source: source, line: 1, column: 1}
	var tokens []token
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tok, err := scan.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.kind == end {
			return tokens, nil
		}
	}
}

func (l *lexer) peek() rune {
	if l.offset == len(l.source) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(l.source[l.offset:])
	return r
}

func (l *lexer) take() rune {
	r, size := utf8.DecodeRuneInString(l.source[l.offset:])
	l.offset += size
	l.column++
	return r
}

func (l *lexer) next() (token, error) {
	for {
		switch l.peek() {
		case ' ', '\t':
			l.take()
		case '#':
			for l.peek() != -1 && l.peek() != '\r' && l.peek() != '\n' {
				l.take()
			}
		default:
			return l.scanToken()
		}
	}
}

func (l *lexer) scanToken() (token, error) {
	at := Span{Start: l.offset, End: l.offset, Line: l.line, Column: l.column}
	if l.peek() == -1 {
		return token{kind: end, span: at}, nil
	}

	first := l.take()
	var kind tokenKind
	switch {
	case first == '\r' || first == '\n':
		if first == '\r' && l.peek() == '\n' {
			l.take()
		}
		l.line++
		l.column = 1
		kind = newline
	case isLetter(first):
		for isLetter(l.peek()) || isDigit(l.peek()) || l.peek() == '_' {
			l.take()
		}
		kind = word
	case isDigit(first):
		l.digits()
		if l.peek() == '.' {
			l.take()
			l.digits()
		}
		if l.peek() == 'e' || l.peek() == 'E' {
			l.take()
			if l.peek() == '+' || l.peek() == '-' {
				l.take()
			}
			l.digits()
		}
		kind = number
	case first == '"':
		for {
			switch l.peek() {
			case -1, '\n', '\r':
				at.End = l.offset
				return token{}, problem(at, "This text needs a closing double quote.")
			case '"':
				l.take()
				kind = quoted
			case '\\':
				l.take()
				if l.peek() != -1 && l.peek() != '\n' && l.peek() != '\r' {
					l.take()
				}
			default:
				l.take()
			}
			if kind == quoted {
				break
			}
		}
	case strings.ContainsRune("+-*/%^=<>!(),", first):
		next := l.peek()
		if (first == '/' && next == '/') ||
			(strings.ContainsRune("<>!", first) && next == '=') {
			l.take()
		}
		kind = symbol
	default:
		at.End = l.offset
		return token{}, problem(at, "Dinoscript doesn't recognize %q here.", first)
	}
	at.End = l.offset
	return token{kind: kind, text: l.source[at.Start:at.End], span: at}, nil
}

func (l *lexer) digits() {
	for isDigit(l.peek()) {
		l.take()
	}
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isLetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'
}
