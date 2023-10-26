# crontab — Coding Challenge #32

A self-contained cron tool written in Go (stdlib only). It parses and validates
standard five-field cron expressions, translates them to plain English,
computes next fire times with an injectable clock, and runs jobs from a crontab
file via a daemon loop.

This implements John Crickett's [Coding Challenge #32](https://codingchallenges.substack.com/p/coding-challenge-32-crontab-tool)
(parse → validate → translate) plus the scheduler extension (next-run, daemon,
job management).

## Build

```sh
go build -o crontab ./cmd/crontab
```

Single static binary, no runtime dependencies.

## Usage

```
crontab validate "<expr>"                                   # parse + validate
crontab describe "<expr>"                                   # plain-English description
crontab next     "<expr>" [--from <RFC3339>] [-n N] [--tz Z] # next N fire times
crontab list     [--file <path>] [--tz Z]                   # list jobs + next fire + description
crontab add      "<expr>" "<command>" [--file <path>]       # validate + append a job
crontab run      [--file <path>] [--tz Z] [--once]          # daemon: schedule + run jobs
crontab version
```

Global file resolution: `--file`, else `$CRONTAB_FILE`, else `./crontab`.

Exit codes: `0` success · `1` validation/parse error · `2` usage error ·
`3` runtime/daemon error.

### Examples

```sh
$ crontab describe "0 9 * * SAT"
At 09:00, only on Saturday

$ crontab next "*/15 9-17 * * 1-5" --from 2026-06-26T08:50:00Z -n 3
2026-06-26T09:00:00Z
2026-06-26T09:15:00Z
2026-06-26T09:30:00Z

$ crontab add "0 2 * * *" "/usr/local/bin/backup.sh" --file ./crontab
added: 0 2 * * *	/usr/local/bin/backup.sh   (At 02:00 every day)
```

## Cron syntax

```
┌───────── minute        (0-59)
│ ┌─────── hour          (0-23)
│ │ ┌───── day of month  (1-31)
│ │ │ ┌─── month         (1-12 or JAN-DEC)
│ │ │ │ ┌─ day of week   (0-6 or SUN-SAT; 7 = Sunday too)
│ │ │ │ │
* * * * *  <command>
```

Per field: `*` (all), `,` (list), `-` (range), `/` (step). Names are
case-insensitive (`jan`, `MON-FRI`). GNU `a/s` (start to domain max, stepped) is
supported. Macros: `@yearly`/`@annually`, `@monthly`, `@weekly`, `@daily`/`@midnight`,
`@hourly`.

### Notable semantics

- **Day-of-month vs day-of-week.** If **both** fields are restricted (neither is
  `*`), a job fires when **either** matches — the classic Vixie-cron OR rule.
  `0 0 13 * FRI` fires on the 13th *or* any Friday. If only one is restricted,
  that field governs.
- **`7` = Sunday**, normalised to `0`.
- **Ranges do not wrap.** `5-1` is an error; write `22-23,0-2` instead of `22-2`.
- **Steps must be ≥ 1.** `*/0` is an error.
- **Calendar gaps are skipped.** `0 0 29 2 *` only fires in leap years;
  `0 0 31 * *` skips 30-day months. An impossible schedule such as `0 0 30 2 *`
  reports "schedule never fires".
- **Time zones / DST.** Next-run runs in an explicit `*time.Location`. A job at a
  local minute skipped by spring-forward does not fire that day; a minute that
  occurs twice at fall-back fires once (first occurrence).
- **Unsupported.** `L`, `W`, `?`, `#` and `@reboot` return a clear
  "not supported in this version" error rather than crashing.

## Architecture

A pure, side-effect-free **core** (parser → matcher → next-run → translator) is
separated from the **I/O shell** (CLI, file store, daemon, process runner). The
core never reads the clock, sleeps, or spawns processes.

```
cmd/crontab/main.go          CLI entrypoint + subcommand dispatch
internal/cron/               parse.go, schedule.go, describe.go, macros.go
internal/clock/              Clock interface; Real + Fake (advanceable)
internal/job/                Job + crontab-file load/add
internal/scheduler/          daemon loop; Runner interface; Exec + fakes
```

The daemon's `Clock` and `Runner` are injected interfaces. In production they are
the real wall clock and an `os/exec` shell runner; in tests they are a
deterministic `FakeClock` (advanced programmatically) and a recording fake
runner — so the scheduler loop runs to completion instantly.

### Daemon behaviour

- Each tick sleeps until the next minute any job is due, then runs every due job
  in its own goroutine.
- **Overlap:** a job is not restarted while a previous run is still active
  (configurable).
- **Missed ticks:** after a clock jump, each job fires at most once on wake
  (tracked by last-fired minute) — no storm.
- **Failures** are logged with exit code; the daemon keeps running.
- **SIGINT/SIGTERM** cancels scheduling and lets in-flight jobs drain.

## Tests

```sh
go test ./...
```

Deterministic and fast — **no real sleeping, no wall-clock reads, no real
subprocesses**. Coverage includes parser valid/invalid tables, next-run cases
(steps, ranges, lists, named months/weekdays, month/year rollover, leap-year
Feb 29, DOM/DOW OR, short-month skipping, DST spring-forward/fall-back,
impossible schedules), natural-language goldens, scheduler loop / overlap /
cancellation / missed-tick behaviour, and crontab-file round-trips.
