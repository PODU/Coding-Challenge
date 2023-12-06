# netmodel — Network Modelling Tool (Coding Challenge #38)

A deterministic, offline command-line tool for modelling an IP network:

1. **Load topology** from a links CSV (bidirectional links, symmetric capacity).
2. **Route** between nodes using OSPF-style shortest paths (lowest total link
   weight) via Dijkstra.
3. **Load traffic demands** from a demands CSV.
4. **Model flow**: route every demand and report per-link load and utilization
   (utilization may exceed 100% — over-subscription is reported, not rejected).
5. **Worst-case single-link-failure analysis**: remove each link in turn,
   re-route all demands, and rank the failures worst-first.

Pure Python 3.11+, **no third-party runtime dependencies**.

## Install

```bash
python -m pip install -e .
```

This exposes a `netmodel` console script. You can also run the package directly
without installing:

```bash
PYTHONPATH=src python -m netmodel <command> ...
```

## Usage

```
netmodel report  --links LINKS.csv --demands DEMANDS.csv [--out report.csv]
netmodel path    --links LINKS.csv --src A --dst D
netmodel failure --links LINKS.csv --demands DEMANDS.csv [--out failure.csv]
```

### Examples (using the bundled sample data)

```bash
netmodel report  --links data/links.csv --demands data/demands.csv
netmodel path    --links data/links.csv --src A --dst E
netmodel failure --links data/links.csv --demands data/demands.csv
```

`report` prints a link-utilization section and a route section:

```
# Link Utilization
LinkId,Start,End,Capacity,Load,UtilizationPct,OverCapacity
L1,A,B,100,90,90.00,false
L2,B,D,100,110,110.00,true
...
# Routes
Source,Destination,Demand,Path,Routed
A,D,60,A->B->D,yes
...
```

`failure` prints the ranked CSV to stdout (worst first) and a one-line worst-case
summary to stderr:

```
FailedLinkId,UnroutableDemands,OverCapacityLinks,MaxUtilizationPct
L6,3,0,60.00
L2,0,3,120.00
...
```

## Input formats

**Links CSV** (header required): `LinkId,Start,End,Capacity,Weight`

| Column   | Rules                                              |
|----------|----------------------------------------------------|
| LinkId   | unique; duplicates rejected                        |
| Start    | node name (created if unseen)                      |
| End      | node name; must differ from Start (no self-loops)  |
| Capacity | `> 0` (symmetric both directions)                  |
| Weight   | `>= 0` (OSPF cost; zero allowed)                   |

**Demands CSV** (header required): `Source,Destination,Demand`

| Column      | Rules                                                       |
|-------------|-------------------------------------------------------------|
| Source      | node name; need not exist in the topology (then unroutable) |
| Destination | node name                                                   |
| Demand      | `>= 0` traffic units                                        |

Column order is flexible; extra columns are ignored. BOM, surrounding
whitespace, and blank lines are tolerated.

## Output / behaviour notes

- **Link utilization** = `Load / Capacity * 100`, rounded to 2 decimals at the
  reporting boundary; `OverCapacity` is `true` only when strictly `> 100%`.
- **Failure ranking** order (the challenge's tie-break): most un-routable
  demands, then most over-capacity links, then highest single-link utilization,
  with `FailedLinkId` as a final stable key.
- **Determinism**: equal-cost (ECMP) paths are broken by choosing the
  lexicographically smallest node sequence. v1 selects a single path and does
  **not** split traffic across equal-cost paths (documented limitation). Re-running
  any command on the same inputs yields byte-identical output.
- **Exit codes**: `0` success · `1` bad CLI usage · `2` malformed/invalid input
  (with a row-numbered message).

## Project layout

```
38-network-modelling/
├── pyproject.toml
├── README.md
├── data/                  # sample input CSVs
│   ├── links.csv
│   └── demands.csv
├── src/netmodel/
│   ├── model.py           # Node, Link, Network
│   ├── topology_loader.py # Step 1: parse links CSV
│   ├── demands_loader.py  # Step 3: parse demands CSV
│   ├── routing.py         # Step 2: Dijkstra (OSPF lowest-weight)
│   ├── flow.py            # Step 4: route demands, accumulate load
│   ├── failure.py         # Step 5: single-link failure sweep + ranking
│   ├── report.py          # CSV report rendering
│   └── cli.py             # argparse entry point
└── tests/                 # pytest suite + hand-verifiable fixtures
```

## Tests

```bash
python -m pip install pytest
python -m pytest
```

The suite covers Dijkstra correctness (lowest-weight vs fewest-hops, zero-weight
links, excluded-link rerouting, ECMP tie-break), demand routing and utilization
math, failure ranking and its tie-breaks, CSV parsing/validation, CLI
integration, and edge cases (disconnected graph, unroutable demands,
over-capacity, source==destination, parallel links).
