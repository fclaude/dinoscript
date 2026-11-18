package dino

import (
	"cmp"
	"math"
	"math/big"
	"strconv"
)

type valueKind uint8

const (
	noValue valueKind = iota
	intValue
	decimalValue
	boolValue
	textValue
)

type value struct {
	kind valueKind
	num  int64
	real float64
	flag bool
	text string
}

func integer(n int64) value   { return value{kind: intValue, num: n} }
func decimal(n float64) value { return value{kind: decimalValue, real: n} }
func boolean(b bool) value    { return value{kind: boolValue, flag: b} }

func (v value) String() string {
	switch v.kind {
	case intValue:
		return strconv.FormatInt(v.num, 10)
	case decimalValue:
		return strconv.FormatFloat(v.real, 'g', -1, 64)
	case boolValue:
		return strconv.FormatBool(v.flag)
	default:
		return v.text
	}
}

func (v value) isNumber() bool { return v.kind == intValue || v.kind == decimalValue }

func (v value) float() float64 {
	if v.kind == intValue {
		return float64(v.num)
	}
	return v.real
}

func overflow(at Span) error {
	return problem(at, "The result is too big for this number type. Try smaller numbers.")
}

func zeroDivisor(at Span) error {
	return problem(at, "You can't divide by zero. Change the number on the right.")
}

func unary(op token, right value) (value, error) {
	if op.text == "not" {
		if right.kind != boolValue {
			return value{}, problem(op.span, "not needs true or false.")
		}
		return boolean(!right.flag), nil
	}
	if !right.isNumber() {
		return value{}, problem(op.span, "%s needs a number here.", op.text)
	}
	if op.text == "+" {
		return right, nil
	}
	if right.kind == intValue {
		if right.num == math.MinInt64 {
			return value{}, overflow(op.span)
		}
		return integer(-right.num), nil
	}
	return decimal(-right.real), nil
}

func (r *runner) binary(expr *expression, locals map[string]value) (value, error) {
	op := expr.at
	left, err := r.requireValue(expr.left, locals)
	if err != nil {
		return value{}, err
	}
	if op.text == "and" || op.text == "or" {
		if left.kind != boolValue {
			return value{}, problem(op.span, "%s needs true or false on both sides.", op.text)
		}
		if op.text == "and" && !left.flag || op.text == "or" && left.flag {
			return left, nil
		}
		right, err := r.requireValue(expr.right, locals)
		if err == nil && right.kind != boolValue {
			err = problem(op.span, "%s needs true or false on both sides.", op.text)
		}
		return right, err
	}
	right, err := r.requireValue(expr.right, locals)
	if err != nil {
		return value{}, err
	}
	switch op.text {
	case "=", "!=", "<", "<=", ">", ">=":
		return compare(op, left, right)
	default:
		return arithmetic(op, left, right)
	}
}

func compare(op token, left, right value) (value, error) {
	var order int
	switch {
	case left.isNumber() && right.isNumber():
		order = compareNumbers(left, right)
	case op.text == "=" || op.text == "!=":
		equal := left.kind == right.kind && left.flag == right.flag && left.text == right.text
		return boolean(equal == (op.text == "=")), nil
	default:
		return value{}, problem(op.span, "%s needs a number on each side.", op.text)
	}
	switch op.text {
	case "=":
		return boolean(order == 0), nil
	case "!=":
		return boolean(order != 0), nil
	case "<":
		return boolean(order < 0), nil
	case "<=":
		return boolean(order <= 0), nil
	case ">":
		return boolean(order > 0), nil
	default:
		return boolean(order >= 0), nil
	}
}

func compareNumbers(left, right value) int {
	if left.kind == right.kind {
		if left.kind == intValue {
			return cmp.Compare(left.num, right.num)
		}
		return cmp.Compare(left.real, right.real)
	}
	if left.kind == intValue {
		var whole, fractional big.Rat
		return whole.SetInt64(left.num).Cmp(fractional.SetFloat64(right.real))
	}
	return -compareNumbers(right, left)
}

func arithmetic(op token, left, right value) (value, error) {
	if !left.isNumber() || !right.isNumber() {
		return value{}, problem(op.span, "%s needs a number on each side.", op.text)
	}
	if left.kind == intValue && right.kind == intValue {
		if op.text != "/" && (op.text != "^" || right.num >= 0) {
			return wholeOperation(op, left.num, right.num)
		}
	}
	a, b := left.float(), right.float()
	var result float64
	switch op.text {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*":
		result = a * b
	case "/":
		if b == 0 {
			return value{}, zeroDivisor(op.span)
		}
		result = a / b
	case "^":
		result = math.Pow(a, b)
	default:
		return value{}, problem(op.span, "%s needs a whole number on each side.", op.text)
	}
	if math.IsNaN(result) {
		return value{}, problem(op.span, "This power doesn't give a real number. Try a different exponent.")
	}
	if math.IsInf(result, 0) {
		return value{}, overflow(op.span)
	}
	return decimal(result), nil
}

func wholeOperation(op token, a, b int64) (value, error) {
	var result int64
	var valid = true
	switch op.text {
	case "+":
		result = a + b
		valid = !(b > 0 && result < a || b < 0 && result > a)
	case "-":
		result = a - b
		valid = !(b > 0 && result > a || b < 0 && result < a)
	case "*":
		result, valid = multiply(a, b)
	case "^":
		result, valid = wholePower(a, b)
	case "//", "%":
		if b == 0 {
			return value{}, zeroDivisor(op.span)
		}
		if op.text == "%" {
			return integer(a % b), nil
		}
		result = a / b
		valid = a != math.MinInt64 || b != -1
	}
	if !valid {
		return value{}, overflow(op.span)
	}
	return integer(result), nil
}

func multiply(a, b int64) (int64, bool) {
	if a == math.MinInt64 && b == -1 || b == math.MinInt64 && a == -1 {
		return 0, false
	}
	product := a * b
	return product, b == 0 || product/b == a
}

func wholePower(base, exponent int64) (int64, bool) {
	var result int64 = 1
	for exponent > 0 {
		if exponent%2 == 1 {
			var valid bool
			result, valid = multiply(result, base)
			if !valid {
				return 0, false
			}
		}
		exponent /= 2
		if exponent > 0 {
			var valid bool
			base, valid = multiply(base, base)
			if !valid {
				return 0, false
			}
		}
	}
	return result, true
}
