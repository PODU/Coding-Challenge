# Forth Interpreter

A small, production-ready interpreter for **Forth** — a stack-oriented,
concatenative language that uses Reverse Polish Notation. It offers two
front ends: an interactive **REPL** and a **script-file runner**.

This is an implementation of [John Crickett's Coding Challenge #95](https://codingchallenges.substack.com/p/coding-challenge-95-forth-interpreter).

## Features

- Integer literals pushed onto a single data stack (signed ints).
- Arithmetic: `+ - * / mod` (`/` is floor division toward negative infinity;
  `mod` is consistent with it). Division/modulo by zero is an error.
- Stack ops: `dup drop swap over rot`.
- Output: `.` `emit` `cr` `." text"`.
- Word definitions: `: name ... ;` with `( ... )` comments.
- Comparison/logic: `< > = <>` and `and or invert` (Forth flags: `0` false,
  `-1` true).
- Control flow: `IF ... ELSE ... THEN` and `limit start DO ... LOOP` with `I`,
  fully nestable.
- `bye` to exit. Word lookups are **case-insensitive** (`DUP` == `dup`).
- Clear errors for stack underflow, unknown words, division by zero, and
  unbalanced `:`/`IF`/`DO` or unterminated `."`/`(`.

## Install

Requires Python 3.10+.

```bash
pip install -e .          # installs the `forth` console script
pip install -e .[test]    # also installs pytest
```

## Usage

### REPL

```text
$ forth                 # or:  python -m forth_interpreter
<0> ok> 1 2 3
<3> 1 2 3 ok> + + .
6 <0> ok> bye
```

The stack summary `<depth> v1 v2 …` is printed before each `ok>` prompt, giving
immediate feedback. A failed line prints a diagnostic and the loop continues
(the stack may retain partial effects — standard Forth behavior).

### Script file

```bash
forth examples/fibonacci.fth     # 0 1 1 2 3 5 8 13 21 34
forth examples/fizzbuzz.fth      # 1 2 Fizz 4 Buzz Fizz 7 8 Fizz Buzz ... FizzBuzz
```

A script runs top to bottom; definitions persist; output goes to stdout. An
uncaught error prints to stderr and exits non-zero.

### Library

The interpreter core is pure (`input -> final stack / output`) and independent
of the REPL/CLI, so it is easy to embed and test:

```python
import io
from forth_interpreter import Interpreter

interp = Interpreter(out=io.StringIO())
interp.run(": square dup * ;  5 square .")
print(interp.out.getvalue())   # "25 "
print(interp.stack)            # []
```

## Architecture

```
source text -> Tokenizer -> Compiler -> instruction lists -> Execute -> Data stack
                                            (+ Dictionary of words)
```

| Module | Responsibility |
|--------|----------------|
| `tokenizer.py` | Whitespace lexing; captures `." ..."` strings, drops `( ... )` comments. |
| `compiler.py`  | Recursively compiles tokens into nested instructions; handles `IF`/`DO`. |
| `instructions.py` | `Push`, `Call`, `PrintStr`, `If`, `Loop` instruction types. |
| `interpreter.py` | Data stack, dictionary, built-in words, execution loop, `run()`. |
| `cli.py` | REPL and script runner (`main`, the `forth` entry point). |

Control-flow words (`IF/ELSE/THEN`, `DO/LOOP`) are **compiler directives**: they
never appear at run time. The compiler consumes them to build nested `If`/`Loop`
instructions, so execution is a simple recursive walk and nesting is free.
User-defined words are **late-bound by name**, so a word can call itself and
redefinitions shadow earlier ones predictably.

## Examples explained

`examples/fibonacci.fth` keeps the pair `a b` on the stack. Each loop step does
`over .` (print `a`) then `swap over +`, turning `a b` into `b a+b` — the next
Fibonacci pair.

## Tests

```bash
python -m pytest
```

The suite (`tests/test_forth.py`) is fully deterministic: each case runs a
program string on a fresh `Interpreter` with captured output and asserts the
final stack and/or stdout. It covers arithmetic, stack ops, definitions and
comments, `IF/ELSE/THEN` (both branches), nested definitions, `DO/LOOP` with
`I`, nested control flow, the Fibonacci/FizzBuzz example files, and every
documented error case.
