package dino

import (
	"context"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type statement struct {
	start      token
	name       token
	parameters []token
	value      *expression
	body       []statement
	alternate  []statement
}

type expression struct {
	at          token
	literal     value
	call        bool
	args        []*expression
	left, right *expression
}

const maxArguments = 64

type parser struct {
	ctx            context.Context
	tokens         []token
	pos            int
	depth          int
	insideFunction bool
	err            error
}

func parse(ctx context.Context, source string) ([]statement, error) {
	tokens, err := lex(ctx, source)
	if err != nil {
		return nil, err
	}
	p := parser{ctx: ctx, tokens: tokens}
	program := p.block(false)
	if p.err == nil && p.peek().kind != end {
		p.fail(p.peek(), "There is no matching block for %q.", p.peek().text)
	}
	return program, p.err
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) next() token {
	current := p.peek()
	if current.kind != end {
		p.pos++
	}
	return current
}

func (p *parser) accept(text string) bool {
	if p.peek().text != text {
		return false
	}
	p.next()
	return true
}

func (p *parser) fail(t token, message string, args ...any) {
	if p.err == nil {
		p.err = problem(t.span, message, args...)
	}
}

func (p *parser) require(text string) {
	if !p.accept(text) {
		p.fail(p.peek(), "Add %q here.", text)
	}
}

func (p *parser) lineBreak(allowEnd bool) {
	if p.peek().kind == newline {
		p.next()
		return
	}
	if allowEnd && p.peek().kind == end {
		return
	}
	p.fail(p.peek(), "Start a new line here.")
}

func (p *parser) enter() bool {
	if p.err != nil {
		return false
	}
	if err := p.ctx.Err(); err != nil {
		p.err = err
		return false
	}
	if p.depth == 128 {
		p.fail(p.peek(), "This code is nested too deeply. Try breaking it into smaller pieces.")
		return false
	}
	p.depth++
	return true
}

func (p *parser) block(nested bool) []statement {
	if !p.enter() {
		return nil
	}
	defer func() { p.depth-- }()
	var statements []statement
	for p.err == nil {
		tok := p.peek()
		switch {
		case tok.kind == end || tok.text == "end" || tok.text == "else":
			return statements
		case tok.kind == newline:
			p.next()
		default:
			statements = append(statements, p.statement(nested))
		}
	}
	return statements
}

func (p *parser) statement(nested bool) statement {
	s := statement{start: p.peek()}
	switch s.start.text {
	case "let":
		p.next()
		s.name = p.name()
		p.require("=")
		s.value = p.expression(1)
	case "output":
		p.next()
		if !p.insideFunction {
			p.fail(s.start, "Use output inside a function. Use print to show a result here.")
		}
		s.value = p.expression(1)
	case "if", "while":
		p.next()
		s.value = p.expression(1)
		if s.start.text == "if" {
			p.require("then")
		} else {
			p.require("do")
		}
		p.lineBreak(false)
		s.body = p.block(true)
		if s.start.text == "if" && p.accept("else") {
			p.lineBreak(false)
			s.alternate = p.block(true)
		}
		p.closeBlock(s.start)
	case "function":
		p.next()
		if nested {
			p.fail(s.start, "Move this function outside the surrounding block.")
		}
		s.name = p.name()
		p.require("(")
		if !p.accept(")") {
			seen := make(map[string]bool)
			for p.err == nil {
				parameter := p.name()
				if len(s.parameters) == maxArguments {
					p.fail(parameter, "This function has too many parameters. Split it into smaller functions.")
					break
				}
				if seen[parameter.text] {
					p.fail(parameter, "This function already has a parameter named %q.", parameter.text)
				}
				seen[parameter.text] = true
				s.parameters = append(s.parameters, parameter)
				if !p.accept(",") {
					break
				}
			}
			p.require(")")
		}
		p.lineBreak(false)
		p.insideFunction = true
		s.body = p.block(true)
		p.insideFunction = false
		p.closeBlock(s.start)
	default:
		s.value = p.expression(1)
	}
	p.lineBreak(true)
	return s
}

func (p *parser) closeBlock(start token) {
	if !p.accept("end") {
		p.fail(start, "Add %q after the last line inside this block.", "end "+start.text)
		return
	}
	p.require(start.text)
}

func (p *parser) name() token {
	tok := p.next()
	if tok.kind != word || reserved(tok.text) {
		p.fail(tok, "Choose a name starting with a letter here.")
	}
	return tok
}

func reserved(text string) bool {
	switch text {
	case "let", "output", "if", "then", "else", "end", "while", "do", "function",
		"true", "false", "and", "or", "not":
		return true
	default:
		return false
	}
}

const (
	comparisonPower = 4
	signPower       = 7
	powerPower      = 8
)

func bindingPower(text string) int {
	switch text {
	case "or":
		return 1
	case "and":
		return 2
	case "=", "!=", "<", "<=", ">", ">=":
		return comparisonPower
	case "+", "-":
		return 5
	case "*", "/", "//", "%":
		return 6
	case "^":
		return powerPower
	default:
		return 0
	}
}

func (p *parser) expression(minimum int) *expression {
	if !p.enter() {
		return nil
	}
	defer func() { p.depth-- }()
	left := p.primary()
	// A chain of left-associative operators does not consume parser recursion,
	// but still creates a deep tree for the evaluator.
	for count := 0; p.err == nil; count++ {
		op := p.peek()
		strength := bindingPower(op.text)
		if strength < minimum {
			break
		}
		if count == 128 {
			p.fail(op, "This expression is too long. Split it across a few variables.")
			break
		}
		p.next()
		nextMinimum := strength + 1
		if op.text == "^" {
			nextMinimum = strength
		}
		left = &expression{at: op, left: left, right: p.expression(nextMinimum)}
	}
	return left
}

func (p *parser) primary() *expression {
	tok := p.next()
	expr := &expression{at: tok}
	switch {
	case tok.text == "(":
		expr = p.expression(1)
		p.require(")")
	case tok.text == "+" || tok.text == "-":
		if tok.text == "-" && p.peek().text == "9223372036854775808" && p.tokens[p.pos+1].text != "^" {
			p.next()
			expr.literal = integer(math.MinInt64)
			break
		}
		expr.right = p.expression(signPower)
	case tok.text == "not":
		expr.right = p.expression(comparisonPower)
	case tok.text == "true" || tok.text == "false":
		expr.literal = boolean(tok.text == "true")
	case tok.kind == number:
		expr.literal = p.numeric(tok)
	case tok.kind == quoted:
		decoded, err := strconv.Unquote(tok.text)
		if err != nil || !utf8.ValidString(decoded) {
			p.fail(tok, "Check the escape sequence inside this text.")
		}
		expr.literal = value{kind: textValue, text: decoded}
	case tok.kind == word && !reserved(tok.text):
		if p.accept("(") {
			expr.call = true
			if !p.accept(")") {
				for p.err == nil {
					if len(expr.args) == maxArguments {
						p.fail(p.peek(), "This call has too many arguments. Split it into smaller calls.")
						break
					}
					expr.args = append(expr.args, p.expression(1))
					if !p.accept(",") {
						break
					}
				}
				p.require(")")
			}
		}
	default:
		p.fail(tok, "Add a value here, such as a number, text, or a variable name.")
	}
	return expr
}

func (p *parser) numeric(tok token) value {
	if strings.ContainsAny(tok.text, ".eE") {
		n, err := strconv.ParseFloat(tok.text, 64)
		if err != nil || math.IsInf(n, 0) {
			p.fail(tok, "Write a valid decimal number.")
		}
		return decimal(n)
	}
	n, err := strconv.ParseInt(tok.text, 10, 64)
	if err != nil {
		p.fail(tok, "This whole number is too large. Try a smaller number.")
	}
	return integer(n)
}
