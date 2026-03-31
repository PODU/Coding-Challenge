# AI Writing Detector — Coding Challenge #113

A command-line tool that analyses a block of text and estimates how likely it
was **written by an AI (LLM)** rather than a human. It is **rule-based and
statistical** — it scores text against a catalogue of known "LLM tells"
(signature words, structural tics, vague language, over-emphasis) and combines
those with seven classic readability / linguistic statistics into a single
**0–100 AI-likelihood score** with a human-readable report.

There is **no ML model, no network call, and no LLM in the detection path**. The
whole system is **deterministic and fully offline**: the same input always
yields the same score.

> Challenge: <https://codingchallenges.substack.com/p/coding-challenge-113-ai-writing-detector>

## Install

Requires Python 3.11+. From this directory:

```bash
python -m pip install -e .
```

This installs the `aidetector` console script and the `aidetector` package.

## Usage

```bash
# positional text
aidetector "In conclusion, we must delve into this robust, innovative realm."

# from a file
aidetector --file essay.txt

# from STDIN
cat essay.txt | aidetector

# also runnable as a module
python -m aidetector "some text"
```

### Options

| Flag | Description |
|------|-------------|
| `--format {text,json}` | Output format (default: `text`). |
| `--threshold-human N` | "likely human" upper bound, exclusive (default 30). |
| `--threshold-ai N` | "likely AI" lower bound, inclusive (default 60). |
| `--explain` | Verbose report: list every finding with evidence. |
| `--judge {none,mock,anthropic}` | Optional second opinion (default `none`). |
| `--version`, `-h/--help` | Standard. |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success (analysis printed). |
| `1` | Empty / whitespace-only input (graceful message). |
| `2` | Usage / IO error (bad file path, bad flag). |

### Example

```
AI Writing Detector — Report
============================
Characters: 1,564    Words: 228    Sentences: 14

AI-likelihood score: 84 / 100   ->  LIKELY AI

Breakdown:
  [2] Signature words/phrases........... 20 / 20
        - signature phrase "delve into"
        ...
  [6] Linguistic factors................ 18.7 / 25
        - sentence-length variation (cv) = 0.31 (low -> AI-leaning)
        - Flesch-Kincaid grade level = 13.2
        ...
```

`--format json` emits the same data as a machine-readable object
(`{score, label, thresholds, stats, detections:[{name, step, capped_score, cap, findings}]}`).

## How it scores

Five independent, self-capping detectors run over a tokenised `Document`; their
capped contributions are summed and clamped to `[0, 100]`:

| Step | Detector | Detects | Cap |
|------|----------|---------|-----|
| 2 | Signature words/phrases | distinct LLM-favourite terms ("delve into", "robust") | 20 |
| 3 | Structural patterns | rule of three, negative parallelism, outline conclusions, false ranges | 20 |
| 4 | Vague language | vague attributions / superficial analysis / overgeneralization (per-category caps) | 18 |
| 5 | Excessive emphasis | undue emphasis, promotional language, elegant variation | 17 |
| 6 | Linguistic factors | 7 statistical factors (below) | 25 |

**Step 6 factors:** lexical diversity (TTR), sentence-length variation,
passive-voice frequency, transition-word density, Flesch-Kincaid grade level,
punctuation patterns, and rare-word usage. Each maps its measured value through
a banded/piecewise-linear curve into points.

**Classification:** `score < 30` → **likely human**, `30–59` → **possibly AI**,
`>= 60` → **likely AI**. All caps and weights live in `src/aidetector/lexicons.py`
for one-place recalibration.

## Optional LLM judge (off by default)

To honour the optional "LLM behind a mocked interface" guidance *without*
contaminating the deterministic core, an opt-in second opinion is available:

- `--judge=none` (default) — pure offline statistical score; what the tests assert on.
- `--judge=mock` — deterministic offline `MockJudge` (no network).
- `--judge=anthropic` — real `AnthropicJudge` targeting **`claude-opus-4-8`**.
  This is the **only** code that can touch the network; it is constructed solely
  when you pass this flag *and* `ANTHROPIC_API_KEY` is set, and requires the
  optional extra: `pip install -e '.[anthropic]'`. It is never built in tests.

## Tests

Deterministic and fully offline (the Anthropic judge is never constructed):

```bash
python -m pytest
```

Coverage: tokeniser counts/normalisation, every detector on fixed fixtures
(known counts/scores), per-category caps, threshold boundaries, end-to-end on a
human/AI fixture corpus (human < 30, AI ≥ 60), empty/short-text handling, report
structure, and the mocked judge.

## Edge cases & limitations

- **Empty / whitespace-only** input → graceful message, exit 1.
- **Very short text** (< ~20 words or < 2 sentences) → statistical factors are
  down-weighted and a "low confidence" note is emitted; a score is still produced.
- **Source code / markup** is treated as plain text; it triggers few signature
  patterns and tends to score "likely human" (documented limitation).
- **Non-English text** — the lexicons are English, so the tool under-detects.
- **Smart quotes / em-dashes** are normalised before pattern matching, while the
  raw form is preserved so punctuation factors can still measure them.
- Some terminals on Windows render the `—` in the report title using the local
  codepage; JSON output and file redirection are UTF-8 and unaffected.

## Project layout

```
src/aidetector/
  tokenize.py     Document + word/sentence/syllable splitting
  lexicons.py     all phrase catalogues + weights/caps (data only)
  detectors/      signature, structural, vague, emphasis, linguistic
  scorer.py       aggregation + classification
  report.py       text / JSON rendering
  judge.py        optional LLM judge (mock + Anthropic), off by default
  cli.py          argparse / IO
tests/            pytest suite + human/ and ai/ fixtures
```
