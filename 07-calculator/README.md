# calc — Coding Challenge #7

A command-line arithmetic calculator that evaluates expression strings with
correct **operator precedence**, **parenthetical grouping**, and
**`decimal.Decimal` precision**. Built as a classic
**tokenizer → Pratt parser (AST) → evaluator** pipeline.

## Features

- Binary operators `+ - * /` with correct precedence and left-associativity.
- Parentheses, including nested and redundant groups.
- Unary `+` / `-` (prefix).
- Exponentiation `^` (right-associative): `2 ^ 3 ^ 2 → 512`.
- Functions: `sin`, `cos`, `tan`, `sqrt`, `abs`.
- Constants: `pi`, `e`.
- `decimal.Decimal` arithmetic — `0.1 + 0.2 → 0.3`, not `0.30000000000000004`.
- Typed error hierarchy (`LexError` / `ParseError` / `EvalError`) with a caret
  pointer and distinct CLI exit codes.
- Three modes: single argument, stdin pipe, interactive REPL.

## Requirements

- Python **3.11+** (developed/tested on 3.12).
- No runtime dependencies. Tests use `pytest` + `hypothesis`.

## Install

```bash
# from the 07-calculator/ directory
pip install -e .            # installs the `calc` console script
# or, for the test tooling too:
pip install -e ".[test]"
```

You can also run without installing, using the `src/` layout directly:

```bash
PYTHONPATH=src python -m calc "1 + 1 * 5"
```

## Usage

### 1. Argument mode (one-shot)

```bash
$ calc "(1 + 1) * 5"
10
$ calc "0.1 + 0.2"
0.3
$ calc --precision 4 "1 / 3"
0.3333
```

### 2. Stdin / pipe mode (one expression per line)

```bash
$ printf "10 / (6 - 1)\n1 + 1 * 5\n" | calc --quiet
2
6
```

### 3. Interactive REPL (no arguments)

```bash
$ calc
calc> 1 + 1 * 5
6
calc> (1 * 2) - (3 * 4)
-10
calc> quit
```

### CLI flags

| Flag | Effect |
|------|--------|
| `<expression>` (positional) | Evaluate and print result, then exit. |
| `--precision N` | Round output to N significant digits (default: full Decimal). |
| `--quiet` | Suppress the `calc> ` prompt (script-friendly). |
| `--version`, `--help` | Standard. |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | evaluation error (e.g. division by zero) |
| `2` | parse/lex error (invalid syntax) |
| `64` | usage error |

Errors print a caret pointer at the offending column:

```
$ calc "1 + @"
1 + @
    ^ LexError: unknown character '@'
```

## Examples (edge cases)

| Input | Output |
|-------|--------|
| `3 / 2` | `1.5` |
| `1 + 1 * 5` | `6` |
| `8 - 3 - 2` | `3` (left-assoc) |
| `2 ^ 3 ^ 2` | `512` (right-assoc) |
| `-(2 + 3)` | `-5` |
| `3 * -2` | `-6` |
| `((3))` | `3` |
| `sin(0)`, `cos(0)` | `0`, `1` |
| `1 / 0` | `EvalError: division by zero` |
| `(1 + 2` | `ParseError: ... ')'` |
| `sqrt(-1)` | `EvalError: domain error` |
| `10 ^ 10000000` | `EvalError: overflow` |

## Project layout

```
07-calculator/
├── README.md
├── pyproject.toml
├── src/calc/
│   ├── __init__.py      # public re-exports (evaluate, error types)
│   ├── tokens.py        # Token, TokenType
│   ├── lexer.py         # tokenize() + LexError
│   ├── ast_nodes.py     # Num/Unary/Binary/Call/Const
│   ├── parser.py        # recursive-descent parser + ParseError
│   ├── evaluator.py     # tree walk, FUNCTIONS/CONSTANTS, EvalError
│   ├── errors.py        # CalcError hierarchy
│   ├── api.py           # evaluate(str) -> Decimal, format_result()
│   ├── cli.py           # argparse, stdin, REPL
│   └── __main__.py      # python -m calc
└── tests/
    ├── test_lexer.py
    ├── test_parser.py
    ├── test_evaluator.py
    ├── test_acceptance.py    # challenge golden cases
    ├── test_properties.py    # hypothesis fuzzing vs reference
    └── test_cli.py
```

## Running the tests

```bash
pip install -e ".[test]"
pytest
```

Or without installing:

```bash
pip install pytest hypothesis
PYTHONPATH=src pytest
```

The suite (102 tests) covers the tokenizer, AST shape / precedence /
associativity, the evaluator edge-case table, the golden challenge cases,
property-based fuzzing (random valid trees vs a reference, random strings never
crash, algebraic identities), and the CLI (all three modes + exit codes).

## A note on `1 + + 2`

The spec is internally inconsistent here: it lists `1 + + 2`
as an error, but its own grammar (`unary := ("+" | "-") unary | primary`)
and its reference evaluator (Python `eval`, §7.3) both treat it as
`1 + (+2) = 3`. Honouring the grammar is also required for the edge cases
`3 * -2 → -6` and `- -5 → 5`, which need unary operators after another operator.
This implementation therefore accepts `1 + + 2` and returns `3`. Genuinely
malformed inputs (`1 +`, `* 2`, `1 2`, `()`, unbalanced parens, bad tokens) are
still rejected with a `ParseError`/`LexError`.
