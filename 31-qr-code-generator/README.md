# qrgen — a from-scratch QR code generator

Coding Challenge #31. `qrgen` implements the full **ISO/IEC 18004** QR encoding
pipeline from scratch — data analysis, segment encoding, Reed-Solomon error
correction over GF(256), module placement, data masking, and format/version
information — and renders the result as **PNG**, **SVG**, or **terminal** output.

No third-party QR library is used for *encoding*. A third-party decoder is used
**only in the test suite** to prove the generated symbols scan correctly.

## Features

- Tightest-mode selection: **numeric / alphanumeric / byte / Kanji** (Shift-JIS).
- All four EC levels **L / M / Q / H** and automatic smallest-version selection (1–40).
- Reed-Solomon ECC over GF(256) (primitive `0x11D`), block splitting + interleaving.
- Full function-pattern placement: finders, separators, timing, alignment, dark module.
- Zig-zag data placement with remainder-bit handling.
- All 8 data masks evaluated with the four penalty rules; lowest-penalty mask wins.
- BCH(15,5) format info and BCH(18,6) version info (v ≥ 7).
- 4-module quiet zone; PNG (Pillow), SVG (stdlib), terminal (Unicode half-blocks).
- The generator core is **stdlib-only** (Pillow is imported lazily, only for PNG).

## Install

```bash
python -m pip install -e .            # core (SVG + terminal, no deps)
python -m pip install -e ".[png]"     # + Pillow for PNG output
python -m pip install -r requirements-dev.txt   # + test/decoder stack
```

## CLI usage

```bash
qrgen "HELLO CC WORLD"                       # terminal render to stdout
qrgen "HELLO CC WORLD" -o hello.png          # PNG (format inferred from extension)
qrgen "https://example.com" -e Q -f svg -o site.svg
echo "12345678" | qrgen -m numeric           # read payload from stdin
python -m qrgen "DATA" -e H -s 12 -b 4        # also runnable as a module
```

Key options: `-e/--ec {L,M,Q,H}`, `-m/--mode {auto,numeric,alphanumeric,byte,kanji}`,
`-v/--version {auto,1..40}`, `--mask {auto,0..7}`, `-s/--scale`, `-b/--border`,
`--invert`, `-o/--output`, `-f/--format`. Run `qrgen -h` for the full list.

Exit codes: `0` success, `1` capacity exceeded / illegal input for a forced mode,
`2` argument error.

## Library usage

```python
import qrgen

qr = qrgen.encode("HELLO CC WORLD", ec="M")
print(qr.version, qr.mask, qr.mode)   # 1 2 alphanumeric
print(qr.to_terminal())
qr.to_png("hello.png", scale=10, border=4)
svg = qr.to_svg(scale=10)
```

## Architecture

```
analyze → encode → reed_solomon → matrix → mask → format_info → render
 (mode)   (bits)   (codewords)    (matrix) (best)  (finalize)
                         ▲
                   tables.py (spec constants)
```

| Module | Responsibility |
|--------|----------------|
| `tables.py` | Capacity / block-structure / alignment / char-count tables (derived) |
| `galois.py` | GF(256) exp/log tables and arithmetic |
| `reed_solomon.py` | Generator polynomials and EC codeword computation |
| `analyze.py` | Mode detection, EC/version selection, validation |
| `encode.py` | Mode indicator, char count, data bits, padding, interleave |
| `matrix.py` | Function patterns, reservations, zig-zag data placement |
| `mask.py` | 8 mask patterns, 4 penalty rules, lowest-penalty selection |
| `format_info.py` | BCH(15,5) format + BCH(18,6) version info and coordinates |
| `render.py` | PNG / SVG / terminal renderers |
| `cli.py` | argparse front end |

## Testing

```bash
python -m pytest
```

The suite covers (see `tests/`):

- **Unit / known-vector tests** (stdlib-only, always run): GF(256) arithmetic vs
  brute force; Reed-Solomon generator polynomials and the Thonky "HELLO WORLD"
  1-Q EC vector; mode detection / bit-encoding / padding; all 32 format strings
  and sample version strings; function-pattern placement and capacity tables.
- **Golden-matrix tests** (`test_known_vectors.py`): the data + EC module region
  is compared bit-for-bit against the independent `qrcode` library at the same
  version/EC/mask.
- **Round-trip decode tests** (`test_roundtrip.py`): generated PNGs are decoded
  with an **independent** decoder (zxing-cpp and/or OpenCV `QRCodeDetector`) and
  asserted to equal the original text, across all EC levels, multiple modes, a
  version ≥ 7 (version-info) case, and an error-correction-recovery test that
  corrupts modules and confirms the symbol still scans.
- **CLI tests** (`test_cli.py`): in-process and subprocess invocation, format
  inference, stdin payload, well-formed SVG, and error/exit-code paths.

### Decoder availability / caveats

- Round-trip and golden tests `skipif` their optional dependency is missing, so
  the stdlib-only unit suite always runs.
- **pyzbar** is supported but optional: on the Microsoft Store build of Python it
  fails to load its native `libzbar`/`libiconv` DLLs, so it is treated as
  unavailable and **zxing-cpp + OpenCV** serve as the independent decoders.
- An **empty payload** produces a structurally valid zero-length symbol, but some
  decoders (zxing-cpp, OpenCV) return no result for zero-content QR codes; the
  round-trip tests therefore use non-empty minimal strings and exercise the empty
  case only for structural validity.

## License

MIT.
