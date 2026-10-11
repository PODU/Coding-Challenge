# `ccprox` — Bluetooth LE proximity finder (Coding Challenge #140)

A command-line "warmer / colder" finder for Bluetooth Low Energy devices. It
surveys what is advertising nearby, ranked by signal strength (RSSI),
calibrates a "near" reference for a device, then hunts one target with
proximity bands, a rough distance estimate and optional Geiger-counter clicks.

Python 3.10+ on [`bleak`](https://github.com/hbldh/bleak), which supports
Windows (WinRT), macOS (CoreBluetooth) and Linux (BlueZ). Classic Bluetooth
(BR/EDR) inquiry is a second signal source on Windows and Linux, and a second
machine running `ccprox --serve` turns a hunt into rough two-station
direction finding. A built-in simulator
(`--simulate`) lets you try every mode without a radio.

## Install & run

```sh
pip install -e ".[dev]"
ccprox                          # survey (Ctrl+C to stop)
ccprox --calibrate "My Phone"   # hold it at arm's length, press Enter
ccprox --hunt "My Phone" --sound
ccprox --simulate --hunt SimTag -v   # no radio needed
python -m pytest                # 91 offline tests
```

If `ccprox` isn't on your PATH, use `python -m ccprox` instead.

Survey (live, Windows laptop):

```
  #  NAME                     ADDRESS            RSSI    AVG  BAND      SIGNAL                 AGE    RAW    CNT
  1  22:64:91:74:F7:F5        22:64:91:74:F7:F5   -68  -71.0  MEDIUM    █···················  0.5s     13     13
  2  05:DF:34:8C:BA:ED        05:DF:34:8C:BA:ED   -73  -74.0  FAR       ····················  3.3s      2      2
  3  LE_WH-1000XM3            CC:98:8B:F5:07:43   -66  -74.5  FAR       ····················  4.2s      3      2
  4  01:38:FB:1D:4E:78        01:38:FB:1D:4E:78   -94  -94.0  FAR       ····················  6.5s      1      1
4 device(s)  ·  near -45 dBm / far -72 dBm  ·  readings raw 19, counted 18, discarded 1
```

Hunt (`--simulate --timeout 3`, excerpt; on a terminal the line updates in place):

```
Hunting SimTag [AA:BB:CC:00:00:01]  near -45 dBm / far -72 dBm  — Ctrl+C to stop
Distances are rough RSSI estimates (walls, bodies and antenna orientation easily cause 2x errors). One radio gives distance, not direction.
SimTag               NEAR      ██████████·········· RSSI  -58 dBm  avg  -58.0  ~  2.7 m
...
SimTag               MEDIUM    ···················· RSSI  -73 dBm  avg  -72.0  ~ 13.4 m
!! TARGET LOST: SimTag — no signal for 3s (last avg -72.0 dBm)
Target reacquired: SimTag
SimTag               FAR       ···················· RSSI  -73 dBm  avg  -73.0  ~ 15.1 m
```

## Usage

| Mode | |
|------|--|
| *(default)* | Survey: table of devices, strongest smoothed signal first, redrawn every `--interval`; devices silent for `--timeout` drop off. |
| `--calibrate T` | Sample `T` (name or address) for `--seconds` at arm's length; the median becomes its near reference, saved to `~/.ccprox/calibration.json` (under both name and address) and used automatically by `--hunt`. |
| `--hunt T` | Track only `T`: band, raw and smoothed RSSI, distance estimate, updated per reading. Says **TARGET LOST** when it goes quiet for `--timeout`, and "reacquired" when it's back. |

| Flag | Default | Meaning |
|------|---------|---------|
| `--near dBm` | saved calibration, else `-45` | RSSI at arm's length |
| `--far dBm` | `-72` | "same room, further away" RSSI |
| `--interval s` | `1` | survey refresh |
| `--timeout s` | `10` | drop (survey) / lost (hunt) after this much silence |
| `--start-timeout s` | `30` | hunt/calibrate: exit 1 if the target never appears |
| `--window n` | `5` | readings in the moving average |
| `--dedupe-ms ms` | `200` | identical RSSI repeated this quickly is a cached duplicate |
| `--path-loss n` | `2` | path-loss exponent (2 free space, 2.5–4 indoors) |
| `--near-distance m` | `0.6` | metres the near reference corresponds to |
| `--sound` | off | hunt: clicks, faster (and higher-pitched on Windows) as the signal strengthens; silent when lost |
| `-v`, `--verbose` | off | raw vs counted reading totals |
| `--seconds s`, `--yes`, `--no-save`, `--calibration-file f` | | calibration controls |
| `--duration s` | | stop after this long (scripts, demos) |
| `--log f.csv` | | append every raw reading (`time,address,name,rssi`) for later plotting |
| `--simulate` | | fake devices: one walks toward/away and drops out at 40–52 s; `SimSpeaker` is classic |
| `--sim-offset dB` | `0` | simulate: shift every RSSI (fake a second station's position) |
| `--transport le\|classic\|both` | `le` | radio(s) to scan; `both` adds a `TYPE` (LE/BR) column |
| `--serve HOST:PORT` | | mode: act as remote station B, streaming readings over TCP |
| `--remote HOST:PORT` | | hunt: connect to station B for direction finding |
| `--baseline m` | | hunt `--remote`: metres between A (this machine) and B; required |
| `--remote-near dBm` | A's near | hunt `--remote`: station B's near reference |
| `--station-name s` | hostname | serve: name announced to clients |

Bands: **IMMEDIATE** ≥ near · **NEAR** down to the near/far midpoint · **MEDIUM** down to far · **FAR** below.

Exit status: 0 ok, 1 target not found, 2 usage, 3 adapter missing / off / unresponsive, 4 permission denied, 5 remote station unreachable / port in use, 130 Ctrl+C.

### Classic Bluetooth (`--transport classic|both`)

```
  #  NAME                     ADDRESS           TYPE  RSSI    AVG  BAND      SIGNAL                 AGE    RAW    CNT
  1  cp60                     41:42:ED:EF:8E:B5 BR     -61  -61.0  MEDIUM    ████████············  4.0s      2      2
  2  34:30:4D:02:28:09        34:30:4D:02:28:09 LE     -66  -70.8  MEDIUM    █···················  2.0s      9      8
```

* **Windows**: WinRT association-endpoint watcher (`System.Devices.Aep.SignalStrength`), restarted every 8 s since each run reports a device once.
* **Linux**: BlueZ D-Bus discovery with `Transport=bredr`, RSSI from `PropertiesChanged` (via `dbus-fast`, already a bleak dependency there).
* **macOS**: not supported (inquiry is only reachable through IOBluetooth); exits 2 with a message.

Caveats: inquiry only finds discoverable devices (most phones only while their
Bluetooth settings screen is open), plus paired ones on Windows, whose strength
Windows may serve from its cache. Updates arrive every few seconds, not several
times a second. A dual-mode device appears once per transport.

### Two-station direction (`--serve` / `--remote`)

```sh
# machine B (e.g. kitchen), 4 m from A
ccprox --serve 0.0.0.0:7140
# machine A
ccprox --hunt "My Phone" --remote kitchen-pc:7140 --baseline 4
```

Each station converts its smoothed RSSI to a distance; two circles around two
known points intersect at the target's position **along the A→B line** and a
distance **to one side of it**. Which side is unknowable with two stations
(that needs a third). When noise makes the circles miss, the best-fitting
point on the line is shown instead:

```
Connected to station B 'kitchen' at 127.0.0.1:7140
Hunting SimTag [AA:BB:CC:00:00:01]  near -45 dBm / far -72 dBm  — Ctrl+C to stop
Direction: A = this machine, B = station 'kitchen', 4 m apart.
SimTag               MEDIUM    ██████████·········· RSSI  -60 dBm  avg  -59.0  ~  3.0 m
[●      A├─────────────┤B       ]  beyond A (2.5 m past A, away from B); -2.5 m along A→B, ±1.6 m off the A–B line (side unknown)  (A -59 dBm ~3.0 m, B -66 dBm ~6.7 m)
```

For anything useful: calibrate both stations (`--remote-near` for B), use a
baseline of several metres, and expect metre-scale error. The stream is plain
JSON lines over unauthenticated TCP; only run `--serve` on a network you trust.

**Distance is approximate, and one radio has no sense of direction.** RSSI
tells you how far, not which way: walk a few steps and see whether the signal
rises or falls.

## Status against the challenge

| Step | Done | Verified |
|------|------|----------|
| 0 Environment | ✅ | live: bleak 3.0.2 on Windows 11 sees nearby devices (-64…-94 dBm) |
| 1 Survey | ✅ | live + tests (ranking, refresh, timeout drop) |
| 2 Calibration & bands | ✅ | live calibrate (5 samples, median -65) + tests (4 bands, overrides shift them, persisted reference) |
| 3 Hunt | ✅ | live + tests (filtering, lost/reacquired, start-timeout exit 1) |
| 4 Smoothing & dedupe | ✅ | tests (window average, 200 ms duplicate rule, raw vs counted); simulator injects OS-style cached repeats |
| 5 Audio | ✅ | tests with an injected sink (tempo vs strength, silence after loss); not listened to on hardware |
| 6 Distance | ✅ | tests (model maths); not compared against a tape measure |
| 7 Adapter errors | ✅ | tests inject real `BleakBluetoothNotAvailableError` reasons, D-Bus/WinRT-style messages and a never-answering adapter; not tested by switching the host radio off |
| + | CSV session log, per-device saved calibration, bar-chart display | tests |
| + | Classic BR/EDR as a second signal source | live on Windows (paired `cp60` at -61 dBm in survey and hunt); BlueZ path tested against a fake D-Bus, not on a Linux machine |
| + | Two-station direction finding | tests over real TCP; live with two processes on one PC (real radio both ends), not with two machines in different spots |
