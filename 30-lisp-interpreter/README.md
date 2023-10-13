# cclisp — Coding Challenge #30: Build Your Own Lisp Interpreter

A small, Scheme-flavoured Lisp interpreter in the lineage of Peter Norvig's
*lispy*, written in Python 3.12. It tokenizes S-expression source, reads it into
a nested-list AST, and evaluates that AST against a lexically-scoped environment
chain. First-class closures and recursion (factorial, Fibonacci, counter
closures) work because user functions are ordinary first-class values.

## Features

- **Data types:** integers, floats, strings, symbols, booleans (`t`/`nil`), lists, procedures.
- **Special forms:** `quote` (and the `'` reader macro), `if`, `define`, `defun`, `lambda`, `set!`, `let`, `begin`, `and`, `or`.
- **Builtins:**
  - Arithmetic: `+ - * /`, `mod`/`modulo` (variadic where it makes sense)
  - Comparison: `< <= > >= =` (chained / variadic)
  - Logic & predicates: `not`, `number?`, `symbol?`, `string?`, `list?`, `procedure?`, `null?`, `eq?`, `equal?`
  - Lists: `list`, `cons`, `car`/`first`, `cdr`/`rest`, `append`, `length`
  - I/O & misc: `format`, `print`, `abs`, `min`, `max`
- **First-class closures** with proper lexical scoping + a tail-call loop so tail recursion does not grow the Python stack.
- **Two front-ends:** an interactive REPL (`cclisp>` prompt) and a `.lisp` file runner.
- **Typed error hierarchy:** `LispError` ← `LispSyntaxError`, `LispNameError`, `LispTypeError`, `LispArityError`.

## Project layout

```
30-lisp-interpreter/
├── pyproject.toml            packaging + console script (cclisp) + pytest config
├── README.md
├── src/cclisp/
│   ├── __init__.py           public API: tokenize/parse/standard_env/evaluate/run
│   ├── __main__.py           enables `python -m cclisp`
│   ├── cli.py                REPL + file runner (entry point)
│   ├── tokenizer.py          tokenize(text) -> list[str]
│   ├── reader.py             parse / parse_all / read_from_tokens / atom
│   ├── types.py              Symbol, Lambda, error hierarchy
│   ├── environment.py        Environment(dict) with parent chain + find()
│   ├── evaluator.py          evaluate(x, env), special-form dispatch, lisp_str
│   └── builtins.py           standard_env(): arithmetic/comparison/list/format
└── tests/
    ├── test_tokenizer.py
    ├── test_reader.py
    ├── test_evaluator.py
    ├── test_errors.py
    ├── test_integration.py
    └── examples/{factorial,fib,counter}.lisp
```

## Install

No third-party runtime dependencies — Python 3.12+ only.

```bash
# from the 30-lisp-interpreter/ directory
python -m pip install -e .          # installs the `cclisp` console script
# for the test suite:
python -m pip install -e ".[test]"  # adds pytest
```

You can also run it **without installing** by putting `src/` on the path:

```bash
PYTHONPATH=src python -m cclisp        # macOS / Linux / Git Bash
$env:PYTHONPATH="src"; python -m cclisp  # Windows PowerShell
```

## Usage

### REPL

```text
$ cclisp                 # or: python -m cclisp
cclisp> (+ 1 2)
3
cclisp> (defun doublen (n) (* n 2))
doublen
cclisp> (doublen 21)
42
cclisp> (format t "hi ~A~%" 42)
hi 42
nil
cclisp>                  # Ctrl-D (Linux/macOS) or Ctrl-Z then Enter (Windows) to exit
```

- The prompt is `cclisp> `; multi-line forms are buffered until the parentheses
  balance (continuation prompt `    ... `).
- Errors are caught and printed without crashing the loop.

### Run a file

```bash
cclisp path/to/program.lisp        # or: python -m cclisp path/to/program.lisp
```

All top-level forms are evaluated in order against one shared global
environment, so later forms see earlier `defun`s. Exit code is `0` on success,
`1` on an uncaught Lisp error (message to stderr), `2` if the file can't be opened.

```bash
$ cclisp tests/examples/factorial.lisp
5! = 120
10! = 3628800
```

## Examples

```lisp
; recursion — factorial
(defun fact (n)
  (if (<= n 1) 1 (* n (fact (- n 1)))))
(fact 10)                ; => 3628800

; recursion — fibonacci
(defun fib (n)
  (if (< n 2) n (+ (fib (- n 1)) (fib (- n 2)))))
(fib 10)                 ; => 55

; closures capture their defining scope
(defun adder (n) (lambda (x) (+ x n)))
(define add5 (adder 5))
(add5 100)               ; => 105

; a counter closure with private mutable state
(defun make-counter ()
  (let ((count 0))
    (lambda () (set! count (+ count 1)) count)))
(define c (make-counter))
(c) (c) (c)              ; => 1, 2, 3

; quote, lists, format
'(1 2 3)                 ; => (1 2 3)
(car (list 1 2 3))       ; => 1
(format t "n = ~A~%" 7)  ; prints: n = 7
```

### `format` directives

`(format dest control args...)` — `dest` is `t` (write to stdout, return `nil`)
or `nil` (return the string). Supported directives: `~A` (aesthetic value),
`~S` (quoted/standard), `~%` (newline), `~~` (literal tilde). Unknown directives
pass through literally.

## Truthiness

Only `nil` (and the empty list) are false; **everything else is true, including
`0`**. Comparison and predicate builtins return `t`/`nil`.

## Tests

```bash
python -m pytest          # 92 tests
python -m pytest -v       # verbose
```

The suite covers the tokenizer, reader (including unbalanced-parens → error),
every special form and builtin, closures/recursion (factorial, fib, counter
closure), the full error hierarchy, and end-to-end file/REPL execution with
golden output.
