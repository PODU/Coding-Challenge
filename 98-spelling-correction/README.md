# spellcorrect — Coding Challenge #98

A production-ready **spelling correction** library plus CLI, implementing
[Peter Norvig's "How to Write a Spelling Corrector"][norvig]: learn a
word-frequency model from a corpus, generate edit-distance candidates
(delete / insert / transpose / replace) at distances 1 and 2, filter to known
words, and rank survivors by corpus frequency.

```
argmax_c P(c|w)  ∝  P(c) · P(w|c)
```

approximated by *"of all known candidate words within the smallest edit
distance, choose the one with the highest corpus frequency."*

## Install

Python 3.12+. Pure standard library — no runtime dependencies.

```bash
pip install -e .            # installs the `spellcorrect` console script
pip install -e ".[test]"    # also installs pytest
```

You can also run it without installing, straight from the source tree:

```bash
PYTHONPATH=src python -m spellcorrect.cli --word speling
```

## CLI

```
spellcorrect [--dict PATH | --csv PATH] [--metrics] [--check-only]
             (--word W | --file F | -)
```

| Mode            | Meaning                                            |
|-----------------|----------------------------------------------------|
| `--word W`      | Correct a single word.                             |
| `--file F`      | Correct every whitespace token in file `F`.        |
| `-`             | Read words from stdin (one or many per line).      |

| Dictionary source | Meaning                                                  |
|-------------------|----------------------------------------------------------|
| *(default)*       | Bundled `data/sample_corpus.txt` (tokenized + counted).  |
| `--dict PATH`     | Plain-text corpus (e.g. Norvig's `big.txt`).             |
| `--csv PATH`      | Kaggle-style `word,count` frequency CSV.                 |

| Option         | Meaning                                                     |
|----------------|-------------------------------------------------------------|
| `--metrics`    | Append a `Time: …ms  N words per second` line (Step 5).     |
| `--check-only` | Lint mode: print only misspellings; exit 1 if any are found.|

Output is one tab-separated line per word: `original<TAB>suggestion`. When a
word is already correct, `suggestion == original` (lowercased). Empty and
non-alphabetic tokens are skipped. Exit codes: `0` success, `1` for
`--check-only` with a misspelling, `2` on usage/IO error.

### Examples

```bash
$ spellcorrect --word speling
speling	spelling

$ spellcorrect --word korrekt
korrekt	correct

$ printf "the qick bron fox\n" | spellcorrect -
the	the
qick	quick
bron	brown
fox	fox

$ spellcorrect --metrics --word the
the	the
Time: 0.011000ms  90910.3 words per second
```

## Library API

```python
from spellcorrect import SpellCorrector

sc = SpellCorrector.from_text_file("big.txt")        # plain-text corpus
sc = SpellCorrector.from_frequency_csv("freq.csv")   # Kaggle word,count CSV
sc = SpellCorrector({"the": 100, "cat": 5})          # explicit frequencies

sc.is_known("the")          # Step 2: True
sc.probability("the")       # P(word) = count / N
sc.correct("speling")       # -> "spelling"   (best single correction)
sc.candidates("speling")    # -> {"spelling"} (smallest-distance known set)
sc.edit_distance("korrekt") # -> 2            (0 / 1 / 2 / -1)
```

**Determinism:** ranking is highest frequency first, ties broken by the
lexicographically smallest word, so results never depend on dict/set iteration
order.

## How it works

- **Step 1 — Data processing** (`corpus.py`): lowercase, NFKD-normalize, and
  regex-tokenize `[a-z]+`; count into a `collections.Counter`. The CSV loader
  skips the header, coerces counts to `int`, and sums duplicates.
- **Step 2 — Dictionary lookup**: a word in the frequency table is correct.
- **Step 3 — Single edit distance**: `edits1` builds delete/transpose/replace/
  insert variants; keep the ones in the table.
- **Step 4 — Double edit distance**: `edits2` streams two-edit variants lazily;
  a known distance-1 word always beats any distance-2 word.
- **Step 5 — Metrics**: `--metrics` reports elapsed time and words/sec.

### Complexity (n = word length, a = 26)

- `edits1`: `O(a·n)` candidate strings, `O(a·n²)` to build.
- `edits2`: `O(a²·n²)` candidates — the bottleneck, hence distance is capped
  at 2. Membership filtering is `O(1)` per candidate (hash set); distance-2 is
  streamed via a generator so space stays `O(a·n)`.

## Tests

```bash
python -m pytest          # core suite (deterministic, offline)
python -m pytest -m acceptance -s   # Norvig accuracy (needs big.txt)
```

The core suite (35 tests) is fully deterministic and offline — it uses a fixed
bundled corpus and explicit frequency tables, covering corpus loading, edit
generation, correction behavior (known pass-through, distance-1/2 fixes,
no-candidate echo, frequency ranking, tie-break, lowest-distance precedence),
and the CLI.

## Accuracy

Measured against Norvig's published test sets, training on his `big.txt`
(~6 MB, not bundled). The bundled `acceptance` test reproduces this when you
supply a corpus via `SPELLCORRECT_BIG_TXT=/path/to/big.txt` or by dropping a
`big.txt` into `tests/data/`:

| Test set                  | This implementation | Norvig's documented |
|---------------------------|---------------------|---------------------|
| `spell-testset1.txt` (270)| **74.8%**           | ~75%                |
| `spell-testset2.txt` (400)| **67.5%**           | ~68%                |
| bundled 40-word sample    | **77.5%**           | —                   |

Throughput on this machine was ~80–140 words/sec with the brute-force `edits2`
generator (single-threaded, untuned). For large-scale use, swap `edits2` for a
SymSpell delete-index or BK-tree behind the same API.

## Layout

```
98-spelling-correction/
├── pyproject.toml            # packaging + console script + pytest config
├── conftest.py               # puts src/ on sys.path for `python -m pytest`
├── src/spellcorrect/
│   ├── corpus.py             # Step 1: load + clean -> frequency table
│   ├── corrector.py          # Steps 2-4: SpellCorrector
│   ├── cli.py                # Step 5: argparse front end
│   └── data/sample_corpus.txt# bundled deterministic corpus (package data)
└── tests/                    # pytest suite + Norvig sample test set
```

[norvig]: https://norvig.com/spell-correct.html

## License

MIT.
