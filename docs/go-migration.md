# Go migration and desktop editor plan

Build Dinoscript as an offline desktop app for macOS and Windows. A parent installs it once. A child double-clicks Dinoscript and lands in an editable example with the cursor in the code and **Run** in view. Use QuickBASIC's edit-and-run workflow as the model. Dinoscript is the default language and the basis for the first lessons. Add restricted Scheme as an optional mode once the Dinoscript workflow is ready. [Scheme subset](scheme.md)

## Go implementation

The Go interpreter runs the same programs in the desktop editor and the command-line runner. Tests cover the language rules, bundled examples, and book lessons. [Interpreter](../internal/dino) · [Desktop editor](../internal/desktop) · [Bundled examples](../internal/desktop/examples)

## Dinoscript behavior for the first release

- Run statements and function declarations in source order at the top level. A named function can call itself and becomes available when its declaration runs. Parameters and `let` bindings live in the current function or top-level scope. `if` and `while` share the surrounding scope so `let i = i + 1` updates a loop counter. A function can read top-level bindings and writes its own local bindings.
- Keep `if … then`, `else`, `end if`, `while … do`, `end while`, and `function … end function`. `output value` returns immediately from the current function, including from inside a branch or loop. A function may finish without a value when called as a statement. Using such a call in an expression or writing `output` at the top level is an error.
- Support integers, decimal numbers, booleans, and quoted strings. Parse `-x`, `+x`, and `not` as prefix operators, and accept a missing final newline. Group operators from tightest to loosest as power (`^`, right-associative), numeric sign, multiplication and division (`*`, `/`, `//`, `%`), addition and subtraction, comparisons, `not`, `and`, then `or`. Require booleans for conditions and logical operators. Short-circuit `and` and `or`.
- Use signed 64-bit integers and finite floating-point decimals. `/` gives a decimal result and `//` truncates toward zero for integer operands. `%` produces the corresponding integer remainder. Report arithmetic overflow, division by zero, and invalid operand types at the operator. Pin promotion, comparisons, and exponentiation with examples in the language guide before wiring editor hints to them.
- `print(...)` joins its arguments as text and ends the line. `readint()` consumes one line of program input and reports a clear error for missing or invalid input. Use `#` for comments so `//` remains integer division. Ship a short Dinoscript guide with examples of each construct and common mistakes.

## Go architecture

- Add a root Go module with a `cmd/dinoscript` desktop entry point. Package the interpreter, editor, examples, and help together. Keep the runner independent of the GUI so tests and a developer command can exercise the same implementation.
- Use a hand-written lexer, recursive-descent statement parser, and Pratt expression parser for Dinoscript. Attach source spans to tokens and AST nodes.
- Prototype Unison as the Go desktop toolkit. Verify packaged launch, editable text, cursor jumps, undo, Unicode input, system file dialogs, and screen reader behavior on both platforms before committing to it. Keep toolkit code at the UI boundary. [Unison documentation](https://github.com/richardwilkes/unison#unison)
- Call the interpreter directly in the app process. Run one program at a time on a worker goroutine and deliver batched output, input requests, and diagnostics to the UI thread. Give each run its own environment, cancellation context, and source snapshot. Show diagnostics from that run in Output and enable source jumps while the editor matches its snapshot. Use the same runner contract for Scheme and keep each language's parser and evaluation rules independent.
- Give programs only their in-memory input and an output sink. Apply limits to source and input size, syntax nesting, evaluation steps, call depth, guest allocations, output bytes, and active execution time. Check cancellation during parsing, evaluation, output, and waits for input. Pause the active execution timer while the learner is typing an answer. A stopped or limited run should leave the editor responsive and ready to run again.

## Editor for kids

Use one desktop window with a code pane and an **Output** pane. Put **Run**, **Stop**, and **Examples** within reach and use the system file dialogs for **Open** and **Save**. Keep code and output visible on a small laptop screen.

- Give the editor line numbers, syntax highlighting, block indentation, undo, and adjustable text size. Support **F5** to run and the platform's usual Open and Save shortcuts. Keep focus visible, make Stop reachable by keyboard, and announce run status and the first error to screen readers.
- Show output as it arrives. When `readint()` asks for an answer, show a field in **Output** and let the learner press Enter to submit it. Keep **Stop** available even when the program is waiting for input. Clear the output on the next run.
- Put a short explanation and a next action next to each error. A message such as “Line 4 is missing `end if`. Add it after the last line inside the `if` block.” should take the learner to the relevant code when selected. Use the same feedback for unknown names, wrong numbers of arguments, and exhausted execution limits.
- Recover the current draft when the app reopens. Store recovery drafts in the user's application data directory. Open and save ordinary `.ds` files and protect unsaved edits when opening an example or another file. When Scheme becomes available, remember a draft for each language and select the corresponding mode when opening a `.ds` or `.scm` file.
- Include short examples for printing, decisions, loops, functions, and input. Give each one a small change to try. Use a simple counting loop as the first loop lesson and keep prime and Fibonacci as later examples.

## Mac and Windows releases

| Platform | Release artifact | Install and launch |
| --- | --- | --- |
| macOS | `Dinoscript.app` in a drag-to-install `.dmg`, with Apple silicon and Intel builds | Drag the app into Applications and double-click it in Finder. |
| Windows | Standalone `Dinoscript.exe` for x64 | Copy the executable to a local folder and double-click it in Explorer. The app can also be launched from a shortcut. |

Build and test on both platforms. For distribution, sign and notarize the Mac package and staple its ticket. Sign and timestamp the Windows executable. Verify launch from the downloaded artifacts on standard user accounts with default security settings, then repeat offline. The desktop release includes everything Dinoscript needs beyond the operating system. Confirm the OS versions and processor types on the kids' computers during the prototype. [Apple distribution guidance](https://developer.apple.com/documentation/xcode/packaging-mac-software-for-distribution) · [Microsoft SignTool guidance](https://learn.microsoft.com/en-us/dotnet/framework/tools/signtool-exe)

## Milestones and acceptance checks

1. **Prove desktop launch and a runnable slice.** Build a double-clickable prototype on macOS and Windows. Edit and run `print(2 + 3 * 4)` in the app and see `14`. Introduce a syntax error and verify that selecting it takes the cursor to the correct place. Use the prototype to settle the toolkit before building out the editor.
2. **Finish Dinoscript execution.** Implement variables, control flow, functions, and input. Check function names, nested blocks, `else`, operator precedence, newline variants, and malformed input. Test the prime example with `1`, `2`, `9`, `25`, and `29`. Keep Fibonacci small and check exact output. Exercise timeouts, recursion and output limits, and stopping while waiting for input, then run a valid program after each case.
3. **Finish the editor and package Dinoscript.** Test edit, Run, error repair, Stop, interactive input, Save, and Open. Check Unicode error locations, keyboard navigation, text size, and draft recovery after quitting. On clean Mac and Windows machines, verify that a parent can install the finished artifact and a child can open it offline from Finder or Explorer and run the sample.
4. **Pilot the Dinoscript experience.** Observe a new learner launching the app, changing an example's output, fixing a missing `end if` using the diagnostic, and saving and reopening their work. Treat repeated difficulty with any of those steps as a release issue.
5. **Add restricted Scheme.** Implement the subset with its own conformance cases. Add a Scheme mode, a short sample, and an in-app link to the bundled reference. Verify that both modes use the same output, Stop, and file workflow while keeping drafts and run state independent. Rebuild and test both release artifacts. [Scheme subset](scheme.md)
