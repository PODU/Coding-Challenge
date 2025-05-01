# cctop

Coding Challenge #88 — **Build Your Own `top`**.

`cctop` is a cross-platform terminal process monitor in the spirit of Unix
`top`/`htop`. It runs full-screen in raw mode, repaints on a fixed interval
(default 2s), and shows a summary header plus a sortable, live process table.
`%CPU` is a *rate* — computed from the change in a process's consumed CPU time
between two consecutive samples, exactly as `top` does, not a cumulative total.

## Build & run

```sh
cd 88-top
go mod tidy
go build -o cctop ./cmd/cctop      # cctop.exe on Windows

./cctop                 # live, on this host (Windows / Linux / macOS)
./cctop --demo          # scripted fake data, deterministic
./cctop --once          # one plain-text frame to stdout (pipe / CI friendly)

# cross-compile
GOOS=linux  go build -o cctop-linux ./cmd/cctop
GOOS=darwin go build -o cctop-macos ./cmd/cctop
```

The live source uses
[`github.com/shirou/gopsutil/v3`](https://github.com/shirou/gopsutil), which has
native implementations for Linux (`/proc`), Windows and macOS, so the same
binary works on all three.

## Usage

```
cctop [flags]
  -d, --delay <seconds>    refresh interval (default 2.0)
  -n, --iterations <N>     exit after N refreshes (0 = run forever)
  -s, --sort <key>         initial sort: cpu|mem|pid|time|user|command (default cpu)
      --reverse            start with ascending sort
  -u, --user <name>        only show processes owned by <name>
  -M, --mem-unit <unit>    kib|mib|gib|auto (default auto)
      --solaris            divide %CPU by CPU count (Solaris mode)
      --demo               run against the built-in fake source (no real procs)
      --once               print one plain-text frame and exit (non-TUI)
      --version            print version and exit
  -h, --help               usage
```

### Interactive keys

| Key            | Action                                          |
|----------------|-------------------------------------------------|
| `q` / Ctrl-C / Esc | Quit, restore terminal                      |
| `s`            | Cycle sort column (cpu→mem→pid→time→user→command) |
| `P` `M` `N` `T`| Jump to sort by cPu / Mem / pid(Number) / Time  |
| `r`            | Reverse current sort order                      |
| `e`            | Cycle memory unit (KiB→MiB→GiB→auto)            |
| `space`        | Force an immediate refresh                       |
| `p`            | Pause / resume auto-refresh                      |

## Screen layout

```
cctop - 14:32:07 up 3:21,  load average: 0.42, 0.55, 0.61
Tasks: 214 total, 2 running, 210 sleeping, 0 stopped, 0 zombie
%CPU:   6.3 us,   2.1 sy,  91.6 id
Mem :   15.6Gi total,    9.2Gi used,    6.4Gi free
------------------------------------------------------------------------------
   PID USER        %CPU  %MEM       RES     TIME+  #TH S COMMAND
  1234 aditya      38.4   4.2   512.0Mi   1:02.44   24 R chrome
   910 root         5.1   0.8    96.0Mi   0:11.07    8 S systemd
   ...
 Sort: %CPU(desc)  Unit: auto  Delay: 2.0s   [q]uit [s]ort [r]ev [e]unit [p]ause [space]refresh
```

Numeric columns are right-aligned and fixed-width; `COMMAND` is left-aligned and
truncated with `…` to fit. On platforms without load average (Windows) the
header prints `-`.

## Architecture

Data flows one way: an OS-specific `ProcessSource` produces raw, monotonic
counters → the **pure** `engine` diffs the new sample against the previous one,
computes per-process `%CPU` rates, aggregates the header and sorts → the `tui`
paints. All OS-specific code is isolated behind one interface, so everything the
user sees (sorting, the CPU%-delta arithmetic, byte/`TIME+` formatting, summary
aggregation) is pure, deterministic and cross-platform.

```
cmd/cctop/main.go          flags, wiring, terminal lifecycle, --once
internal/model/            Sample/Process/Snapshot/Summary, enums   (pure)
internal/engine/           diff + %CPU rate, aggregate, sort        (pure)
internal/format/           bytes→KiB/MiB/GiB, secs→TIME+, truncate  (pure)
internal/source/source.go  ProcessSource interface + raw types
internal/source/gopsutil.go default cross-platform live source
internal/source/fake.go     scripted source for --demo and tests
internal/tui/render.go      pure frame rendering (golden-tested)
internal/tui/tui.go         tcell raw-mode interactive loop
```

### The `%CPU` delta (core algorithm)

```
deltaProcCPU = cpuTime[pid].now - cpuTime[pid].prev   // CPU-seconds used
deltaWall    = sampleTime.now   - sampleTime.prev      // wall-clock seconds
cpuPercent   = 100 * deltaProcCPU / deltaWall          // Irix style (can exceed 100)
// --solaris additionally divides by NumCPU
```

Edge rules (all unit-tested): a PID seen for the first time → `0%`; a vanished
PID is dropped; `deltaWall <= 0` → `0%` (no divide-by-zero); a counter that goes
backwards (PID reuse) is clamped to `>= 0`; `%MEM = 100 * RSS / totalPhysical`.

## Tests

```sh
go test ./...           # all green
go test ./... -cover    # engine 93.8%, format 90.6%, source 89.6%
```

All logic tests use injected fixtures via the `fake` source — no real processes,
fully deterministic and cross-platform. They cover the `%CPU`-delta math (known
jiffy deltas → expected %), every sort order (asc/desc + tie-break), byte and
`TIME+` formatting boundaries, summary aggregation (task counts + host CPU%), the
source contract, and a golden render frame. The TUI loop (`tui.go`) is driven by
tcell and exercised manually; its rendering helpers are golden-tested.

> Note: `go test -race` requires cgo, which needs a 64-bit C toolchain. If your
> host only has a 32-bit MinGW gcc, run the suite without `-race`.
