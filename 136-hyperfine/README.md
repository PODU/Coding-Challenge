# `hyperfine` — command-line benchmarking (Coding Challenge #136)

A benchmarking tool in the style of [hyperfine](https://github.com/sharkdp/hyperfine):
it runs commands repeatedly, subtracts the cost of the shell it used to run
them, reports mean / σ / median / range, compares commands against each other
with propagated error margins, and warns when the numbers should not be
trusted.

Go, no dependencies.

## Build & run

```sh
go build -o cchyperfine .
./cchyperfine 'sleep 0.3'
```

```
Benchmark 1: sleep 0.3
  Time (mean ± σ):     304.7 ms ±   1.6 ms    [User: 1.1 ms, System: 1.8 ms]
  Range (min … max):   302.6 ms … 308.4 ms    10 runs
```

With no `-r`, the run count is chosen so the benchmark takes about three
seconds, never fewer than ten runs.

## Usage

```
cchyperfine [OPTIONS] <command>...
```

### Run control

| Flag | Meaning |
|------|---------|
| `-r`, `--runs NUM` | Exact number of timed runs. |
| `-m`, `--min-runs NUM` | Floor for the automatic run count (default 10). |
| `-M`, `--max-runs NUM` | Ceiling for the automatic run count. |
| `-w`, `--warmup NUM` | Untimed runs first, to fill caches. |

### Hooks — never included in the measurement

| Flag | When it runs |
|------|--------------|
| `-s`, `--setup CMD` | Once, before all runs of a command. |
| `-p`, `--prepare CMD` | Before each timed run. |
| `-C`, `--conclude CMD` | After each timed run. |
| `-c`, `--cleanup CMD` | Once, after all runs of a command. |

`-p` and `-C` given once apply to every command; given once per command they
pair up positionally.

### Execution

| Flag | Meaning |
|------|---------|
| `-S`, `--shell SHELL` | Use a different interpreter, e.g. `--shell="zsh -f"`. |
| `-N` | Shorthand for `--shell=none` — execute directly, no shell. |
| `-i`, `--ignore-failure` | Don't abort on a non-zero exit; warn instead. |
| `--input WHERE` | `null`, or a file fed to the command's stdin. |
| `--output WHERE` | `null` (default), `pipe`, `inherit`, or a file path. |
| `--show-output` | Shorthand for `--output=inherit`. |

### Comparison

```sh
cchyperfine -w 3 'grep -R TODO .' 'rg TODO'
```

```
Summary
  rg TODO ran
    4.81 ± 0.18 times faster than grep -R TODO .
```

| Flag | Meaning |
|------|---------|
| `-n`, `--command-name NAME` | Display name for the n-th command. |
| `--reference CMD` | Benchmark `CMD` too and compare everything against it. |
| `--sort METHOD` | `auto` (given order), `command`, `mean-time`. |

### Parameters

```sh
cchyperfine -N -P threads 1 8 -D 2 'build --jobs {threads}'
cchyperfine -L compiler gcc,clang -L opt O0,O2 '{compiler} -{opt} main.c'
```

`-P VAR MIN MAX` sweeps a numeric range (`-D` sets the step; decimals and
negative steps work). `-L VAR a,b,c` sweeps a list and is repeatable, producing
the cartesian product. Both substitute `{VAR}` in the command, in the
`--command-name`, and in the hooks.

### Output and export

| Flag | Meaning |
|------|---------|
| `-u`, `--time-unit UNIT` | `microsecond`, `millisecond`, `second`. |
| `--style TYPE` | `auto`, `basic`, `full`, `nocolor`, `color`, `none`. |
| `--export-json FILE` | Every statistic plus every raw timing and exit code. |
| `--export-csv FILE` | One row per benchmark, with `parameter_*` columns. |
| `--export-markdown FILE` | Comparison table. |
| `--export-asciidoc FILE` | Comparison table. |
| `--export-orgmode FILE` | Comparison table. |

`$HYPERFINE_ITERATION` is set to the zero-based run index for the benchmarked
command and for every hook.

## What makes this more than a stopwatch

**Shell startup is measured and removed.** Before the first benchmark the shell
is invoked 50 times with an empty command; the mean is subtracted from every
later measurement. `timing sh -c "true"` otherwise measures `sh`, not `true`.
Use `-N` to skip both the shell and the correction.

**Four warnings tell you when to distrust the result:**

- *Non-zero exit* — the command failed. Aborts unless you pass `-i`.
- *Under 5 ms* — the shell-spawn correction is itself larger than the signal.
- *Slow first run* — the first sample is an extreme outlier and the maximum:
  a cold cache. Suggests `--warmup`.
- *Statistical outliers* — some sample sits more than ten median-absolute-
  deviations from the median. Something else on the machine interfered.

Outliers are found against the **median** and **MAD**, not the mean and standard
deviation, because an outlier inflates the standard deviation it would be
compared against.

**Relative speeds carry error bars.** `1.02 ± 0.31 times faster` reads as "no
measurable difference"; `4.81 ± 0.18` reads as a real speedup. The margin is the
two relative errors added in quadrature.

## Tests

```sh
go test ./...
go test -cover ./...
```

Statement coverage is 80–96% per package. The suite runs on Windows and Unix
without fixtures — it uses `cmd.exe /c exit 0` or `true` depending on the
platform.

## Windows notes

Go escapes each process argument with MSVC rules, which `cmd.exe` does not
follow, so a command containing a redirection would be mangled. This
implementation builds the `CreateProcess` command line by hand
(`cmd.exe /s /c "…"`) so commands are passed through byte for byte.

Processor times come from `GetProcessTimes` on Windows and `wait4` rusage
elsewhere. Both report the spawned process; on Windows, time spent in
grandchildren of the shell is not attributed back to it, so `User`/`System`
figures there are a lower bound. Wall-clock time — the number the benchmark is
actually about — is unaffected.
