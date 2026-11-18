# Proposed Scheme subset reference

Scheme is an optional mode in the Dinoscript desktop app for learning prefix expressions, functions, recursion, and lists. Use **Scheme** mode for `.scm` files. The forms below define the proposed first release. They draw on R7RS-small, with fixed evaluation order, bounded integers and recursion, and a small set of procedures. [R7RS-small report](https://small.r7rs.org/attachment/r7rs.pdf)

## First program

```scheme
; Add the numbers in a list.
(define (sum numbers)
  (if (null? numbers)
      0
      (+ (car numbers) (sum (cdr numbers)))))

(display (sum (list 2 3 4)))
(newline)
```

Click **Run** to print `9`. Change a number in the list and run it again. Each run starts with fresh definitions. To show a result in **Output**, use `display` and optionally `newline`.

## Values and syntax

- Integers are signed 64-bit decimal values, such as `0`, `12`, and `-3`. A literal may start with `+` or `-`. Out-of-range literals and results are errors.
- Booleans are `#t` and `#f`. Only `#f` counts as false in `if`.
- Strings are double-quoted and may contain Unicode text. Use `\"` for a quote, `\\` for a backslash, `\n` for a newline, and `\t` for a tab. Write line breaks inside a string using `\n`.
- Names are case-sensitive. A name can start with an ASCII letter or `_`, followed by ASCII letters, digits, `_`, `-`, or `?`. The operator names `+`, `-`, `*`, `=`, `<`, `<=`, `>`, and `>=` are also valid names. Spaces, tabs, line breaks, parentheses, and the start of a comment separate tokens. A semicolon starts a comment through the end of the line.
- Lists are immutable and may hold values of different types. Write `(list)` to make an empty list. Functions are values and can be passed to other functions.

## Forms

| Form | What it does |
| --- | --- |
| `(define name expression)` | Evaluates the expression and binds the result at the top level. |
| `(define (name parameter ...) body ...)` | Defines a named function at the top level. Its body can refer to the function by name for recursion. |
| `(lambda (parameter ...) body ...)` | Creates a function that can read the bindings in the scope where it was created. |
| `(let ((name expression) ...) body ...)` | Evaluates the initializers in the surrounding scope and then introduces all the names together for the body. |
| `(if test consequent alternate)` | Evaluates the test, then evaluates and returns the selected branch. Both branches are required. |
| `(begin expression ...)` | Evaluates one or more expressions in order and returns the last value. |
| `(procedure-expression argument ...)` | Evaluates the procedure and then its arguments from left to right, and calls the procedure. |

The program evaluates top-level forms in order. `define` is available only at the top level, and each top-level name can be defined once. Function and `let` bodies contain one or more expressions, returning the last value. Functions take exactly the number of parameters declared. Parameter names and the names in one `let` must be distinct. The form names are reserved. Built-in procedures live in an outer scope, so user bindings can shadow them.

## Built-in procedures

| Procedure | What it does |
| --- | --- |
| `+`, `*` | Add or multiply any number of integers. With zero arguments, the results are `0` and `1` respectively. |
| `-` | Negates one integer or subtracts the rest from the first, left to right. |
| `=`, `<`, `<=`, `>`, `>=` | Compare two integers and return a boolean. |
| `quotient`, `remainder` | Take two integers. The quotient rounds toward zero and the remainder has the dividend's sign or is zero. A zero divisor is an error. |
| `not` | Takes one value and tests whether it is `#f`. |
| `list`, `cons` | Make a list from zero or more values, or put one value at the front of an existing list. |
| `car`, `cdr` | Return the first item or the rest of a nonempty list. Both report an error on an empty list. |
| `null?` | Takes one value and tests whether it is the empty list. |
| `display`, `newline` | Write one value as text or write a line break. `newline` takes zero arguments. Use these procedures for their output. Their return values are unspecified. |

`display` writes strings without quotes, booleans as `#t` or `#f`, integers in decimal, functions as `#<procedure>`, and lists in parentheses with spaces between items. For example, `(display (list 1 #t 3))` prints `(1 #t 3)`.

## Errors and limits

Run reports the location of syntax errors, unknown names, wrong argument counts, and type errors. The runner limits work, recursion depth, memory used by guest values, and output. A recursive call in tail position still counts toward the call depth limit. Use **Stop** to interrupt a run.

The subset excludes `quote` and `'`, symbols as data, mutation such as `set!`, macros, `eval`, `load`, imports, continuations, and the full Scheme numeric tower. The procedures above are the available library. Programs have no access to files, the network, shell commands, or program input.

## Conformance examples

| Program or expression | Expected behavior |
| --- | --- |
| `(display (if #f missing 7))` | Prints `7` because only the selected branch runs. |
| `(define x 2)` followed by `(display (let ((x 3) (y x)) (+ x y)))` | Prints `5` because both initializers see the surrounding scope. |
| `(display ((lambda (x) (* x x)) 4))` | Prints `16`. |
| `(display (quotient -7 3))` followed by `(newline)` and `(display (remainder -7 3))` | Prints `-2` on the first line and `-1` on the second. |
| `(car (list))` | Reports that `car` needs a nonempty list. |
| `(load "lesson.scm")` | Reports that `load` is unavailable in this subset. |
