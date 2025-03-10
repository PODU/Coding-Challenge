# tzc — Time Zone Converter

Coding Challenge #85. A command-line tool that converts a time from a **source
timezone** into **one or more target zones**, correctly accounting for each
zone's UTC offset **and Daylight Saving Time (DST)** at the specific instant.

It also renders the classic "meeting planner" view: an aligned 24-hour grid so
you can eyeball overlapping working hours across zones, and a range view that
shows a working window (e.g. `09:00..17:00`) in every zone.


## Why it is correct everywhere

The binary imports Go's `time/tzdata`, which **embeds the full IANA timezone
database** into the executable. DST rules are therefore correct and identical on
every machine regardless of the host OS's tz data, and the host's `TZ`
environment variable never leaks into results. Zero third-party dependencies;
single static binary.

## Build

```bash
go build ./cmd/tzc        # produces ./tzc (or tzc.exe on Windows)
go test ./...             # run the full deterministic test suite
```

Requires Go 1.24+.

## Usage

```
tzc [flags] <target-zone> [<target-zone> ...]
```

| Flag | Alias | Default | Meaning |
|------|-------|---------|---------|
| `--time` | `-t` | now | Source time to convert (see layouts below). |
| `--from` | `-f` | UTC | Source timezone (IANA name / city / abbreviation). |
| `--format` | | `2006-01-02 15:04` | Output layout: `kitchen`, `24h`, `12h`, `iso`, or a literal Go layout. |
| `--12h` | | off | 12-hour clock with AM/PM. |
| `--grid` | | off | Render the aligned 24-hour planner grid. |
| `--range` | | "" | `START..END` (in source zone) → show both ends per zone. |
| `--no-sort` | | sorted | Keep target order (default sorts west→east by offset). |
| `--list` | | off | List known city aliases + abbreviations, then exit. |
| `--json` | | off | Machine-readable output. |

**Zone resolution** accepts, in order: a full IANA name (`America/New_York`), a
city alias (`new york`, `london`, `tokyo` — case/underscore/space insensitive),
or a common abbreviation (`EST`, `IST`, `JST`, `CET`…). Abbreviations are
inherently ambiguous; the tool uses a documented canonical mapping and prints
which zone it chose (use the full IANA name to override).

**Accepted input time layouts** (tried in order, interpreted in the `--from`
zone): `2006-01-02 15:04`, `2006-01-02T15:04`, `2006-01-02 15:04:05`,
`2006-01-02`, `15:04` (today's date), `3:04PM`, RFC3339, and the literal `now`.

**Exit codes:** `0` ok · `1` bad usage/flags · `2` unknown zone · `3` unparseable time.

## Examples

```bash
# 9:00 New York time, in London and Tokyo
tzc -t "2025-03-09 09:00" -f "new york" london tokyo

# Across the US DST spring-forward (02:30 doesn't exist locally → warned)
tzc -t "2025-03-09 02:30" -f "America/New_York" UTC

# Meeting planner grid, sorted west→east, source = San Francisco
tzc --grid -f "san francisco" london berlin tokyo sydney

# A 09:00..17:00 working window seen from every zone, as JSON
tzc --range "09:00..17:00" -f london new_york tokyo --json

# Discover zones / aliases
tzc --list
```

### Sample output

```
$ tzc -t "2025-03-09 02:30" -f "America/New_York" UTC
warning: "2025-03-09 02:30" does not exist in America/New_York (spring-forward gap); using 03:30 EDT (-04:00)
Source: America/New_York  2025-03-09 03:30
ZONE                LOCAL TIME        OFFSET  DST  Δ vs src
* America/New_York  2025-03-09 03:30  -04:00  DST  +00:00
UTC                 2025-03-09 07:30  +00:00  -    +04:00
```

## DST and edge-case handling

- **Spring-forward gap** — a non-existent local time (e.g. US `02:30` on the
  spring-forward date) is normalised into the post-jump offset and a warning
  names the assumed offset.
- **Fall-back overlap** — an ambiguous local time that occurs twice resolves to
  the earlier offset, with a warning.
- **Half-hour / 45-minute offsets** — `Asia/Kolkata` (+05:30),
  `Asia/Kathmandu` (+05:45) render correctly (offsets tracked in seconds).
- **Cross-day rollover** — a `(+1d)` / `(-1d)` marker is shown when the target
  wall date differs from the source.
- **Southern-hemisphere inverse DST**, historical/LMT offsets, and same
  source/target zones are all handled via the IANA database.

## Architecture

```
cmd/tzc/main.go             # CLI wiring: flag parsing, subcommands, rendering
internal/parser/parser.go   # time string → time.Time in the source zone (+ gap/overlap detection)
internal/zones/zones.go     # token → *time.Location, city/abbrev alias tables, suggestions
internal/convert/convert.go # engine: convert, sort by offset, grid, range
internal/render/render.go   # text / grid / JSON formatters
```

The conversion engine is decoupled from the CLI so the same core could back a
TUI or web frontend that implements the interactive hover/drag planner features.

## Tests

`go test ./...` runs deterministic, host-independent tests covering: golden
conversions across DST boundaries (US Eastern ↔ Europe/London around their
differing DST change dates), the spring-forward gap and fall-back overlap,
half-hour zones, cross-day rollover, zone resolution (aliases, abbreviations,
ambiguity, unknown-zone errors), the planner grid layout, the range view,
JSON-schema stability, round-trip identity, and host-`TZ` independence.
