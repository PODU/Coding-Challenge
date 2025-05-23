# ELIZA — Coding Challenge #89

A faithful, production-ready re-implementation of Joseph Weizenbaum's **ELIZA**
(1966) and its famous **DOCTOR** script — a parody Rogerian psychotherapist that
reflects your statements back as questions. Pure **Python 3 (3.10+), standard
library only**.

ELIZA has no understanding of language. It scans input for keywords, picks the
highest-ranked one, decomposes the sentence around it into captured fragments,
and reassembles a reply from a template — applying pronoun reflection (I→you,
my→your) so the reply reads as a coherent question.

```
YOU:   I am unhappy
ELIZA: I am sorry to hear you are unhappy. Do you think coming here will help you not to be unhappy?
YOU:   My mother never listens to me
ELIZA: Tell me more about your family.
YOU:   It is my fault
ELIZA: Your fault?
```

## Features

- **External, editable DOCTOR script** in JSON (`src/eliza/data/doctor.json`).
- **Ranked keyword scanning** — highest `rank` wins, ties broken by first
  appearance.
- **Decomposition / reassembly** rules with `*` wildcards, `@class` synonym
  matching, and 1-based `(n)` positional placeholders.
- **Pronoun reflection** applied per-token to every captured fragment.
- **Memory queue** with deferred recall (`my …` → "Earlier you said your …").
- **Rule delegation** (`=WHAT`) — a keyword can borrow another's rules.
- **`xnone` fallbacks** ("I see.", "Please go on.", …) when nothing matches.
- **Round-robin reassembly cycling** so repeated input doesn't repeat replies.
- **Pure, deterministic engine** decoupled from the REPL — the engine does zero
  I/O, which makes every test an exact-string assertion.

## Architecture

```
src/eliza/
  script.py   Script model + JSON loader/validator (compiles decomps to regex)
  reflect.py  pronoun/word reflection on token lists
  engine.py   ElizaEngine — pure, deterministic response generator (no I/O)
  cli.py      REPL: banner, prompt loop, exit handling (all I/O lives here)
  data/doctor.json   the bundled classic DOCTOR script
tests/        deterministic pytest suite
```

`ElizaEngine.respond(text) -> str` is a function of (engine state, input). Its
only mutable state is the per-rule reassembly cursors and the memory queue, both
inspectable; with `reassembly="cycle"` (the default) it is fully deterministic.

## Install & run

No dependencies. Run straight from the source tree:

```bash
cd src
python -m eliza
```

Or install the console script:

```bash
pip install -e .
eliza                              # bundled DOCTOR script
eliza --script path/to/my.json     # an alternate script
eliza --random --seed 42           # reproducible random reassembly
```

Exit with `bye`, `goodbye`, `quit`, or `exit` (also Ctrl-D / Ctrl-C).

## Library API

```python
from eliza import ElizaEngine, Script

script = Script.load_default()          # or Script.load("scripts/doctor.json")
engine = ElizaEngine(script)            # deterministic by default

engine.greeting()                       # an `initial` line
engine.respond("I am sad")              # -> reply string
engine.is_quit("bye")                   # -> True
engine.farewell()                       # a `final` line
engine.memory                           # inspectable collections.deque
```

`Script.load` / `Script.loads` raise `ScriptError` (a `ValueError`) on malformed
JSON, an unknown `delegate` target, an unknown `@class`, a missing `xnone`
keyword, or a `(n)` placeholder that exceeds its rule's group count.

## Script format (brief)

```jsonc
{
  "keyword": "my",
  "rank": 2,
  "memory": ["Earlier you said your (2)."],   // pushed onto the recall queue
  "rules": [
    { "decomp": "* my @family *",             // * = wildcard, @family = synonym class
      "reassemb": ["Tell me more about your family.", "Your (2)?"] },
    { "decomp": "* my *",
      "reassemb": ["Your (2)?", "Why do you say your (2)?"] }
  ]
}
```

- `*` matches zero or more words and is captured; a bare word matches literally;
  `@class` matches one member of a `synonyms` class and is captured.
- `(n)` in a `reassemb`/`memory` template is the **n-th capture group** of the
  matched decomposition (counting `*` and `@class` left to right), inserted
  **after pronoun reflection**.
- A rule may carry `"delegate": "<keyword>"` instead of `reassemb` to redirect to
  another keyword's rules (the classic `=WHAT`).

To customise behaviour, copy `src/eliza/data/doctor.json`, edit it, and pass it
with `--script`.

## Tests

```bash
python -m pytest
```

Deterministic suite (25 tests) covering reflection, script validation, keyword
ranking + tie-break, decomposition/reassembly with wildcard capture and
reflection, round-robin cycling, memory push/recall, delegation, `xnone`
fallback cycling, and a CLI smoke test. `mypy --strict` clean.

## Notes / caveats

- The bundled `doctor.json` is a representative DOCTOR subset, not the verbatim
  1966 listing; the engine imposes no fixed keyword set, so it is fully editable.
- `(n)` follows the spec's authoritative **capture-group** numbering (§4.2):
  each `*` and `@class` is one numbered group; literal words do not count. 
- The classic flat Weizenbaum text format is intentionally out of scope; JSON is
  the canonical script format.
