# `fzf` — command-line fuzzy finder (Coding Challenge #137)

A fuzzy finder in the style of [fzf](https://github.com/junegunn/fzf): it reads
items on standard input, narrows them interactively as you type, ranks the
matches the way a person would, and prints what you picked.

Go, no dependencies.

## Build & run

```sh
go build -o ccfzf .
find . -type f | ./ccfzf
```

```
  src/internal/algo/algo.go
  src/internal/matcher/store.go
> src/main.go
  3/1482
> src main
```

Typed characters need only appear in order, so `src main` finds
`src/internal/cmd/main.go`, and the best match is the one against the prompt.

## Usage

```
ccfzf [options]
```

### Search

| Flag | Meaning |
|------|---------|
| `-x`, `--extended` / `+x`, `--no-extended` | Extended-search mode (on by default). |
| `-e`, `--exact` | Terms match exactly instead of fuzzily. |
| `-i`, `--ignore-case` / `+i`, `--no-ignore-case` | Override smart case. |
| `--algo=v1\|v2` | Greedy matcher, or the optimal one (default). |
| `-n`, `--nth=N[,..]` | Match against these fields only. |
| `--with-nth=N[,..]` | Display these fields only. |
| `-d`, `--delimiter=REGEX` | Field separator (default: whitespace). |
| `--tiebreak=CRI[,..]` | `length`, `begin`, `end`, `index` (default `length`). |

### Extended-search syntax

A query is a list of terms, all of which must match.

| Term | Matches |
|------|---------|
| `main` | fuzzily |
| `'main` | exactly, as a contiguous run |
| `^src` | at the start |
| `go$` | at the end |
| `^main$` | the whole item |
| `!test` | items that do *not* match |
| `go$ \| rs$` | either alternative |

```sh
# Go files under src, excluding tests
find . -type f | ./ccfzf -q "^src 'go$ !_test"
```

### Results and interface

| Flag | Meaning |
|------|---------|
| `--no-sort` / `--tac` | Keep the input order / reverse it. |
| `-m`, `--multi[=MAX]` / `+m` | Select several items with `Tab`, optionally capped. |
| `--height=[~]N[%]` | Draw in a window at the bottom instead of the whole screen. |
| `--layout=default\|reverse\|reverse-list`, `--reverse` | Prompt position and list direction. |
| `--info=default\|inline\|hidden` | Where the `matched/total` counts go. |
| `--prompt`, `--pointer`, `--marker`, `--header` | Cosmetics. |
| `--preview=CMD`, `--preview-window=POS[,SIZE[%]]` | Preview pane. |

### Input / output

| Flag | Meaning |
|------|---------|
| `--ansi` | Interpret ANSI colours in the input. |
| `--read0` / `--print0` | NUL-delimited input / output. |
| `--expect=KEYS` | Accept on these keys too, and print which one was used. |
| `-q`, `--query=STR` | Start with a query. |
| `-f`, `--filter=STR` | Print the matches and exit; no interface. |
| `--print-query` | Print the query as the first output line. |
| `-1`, `--select-1` / `-0`, `--exit-0` | Skip the interface when the answer is already decided. |

## Keys

| Key | Action |
|-----|--------|
| `Enter` | Accept |
| `Ctrl-C`, `Ctrl-G`, `Esc` | Abort |
| `↑` / `Ctrl-K` / `Ctrl-P` | Previous match |
| `↓` / `Ctrl-J` / `Ctrl-N` | Next match |
| `PgUp` / `PgDn` | Move a page |
| `Tab` / `Shift-Tab` | Mark / unmark (with `--multi`) |
| `Ctrl-A` / `Ctrl-E` | Start / end of the query |
| `Ctrl-W` / `Ctrl-U` | Delete the previous word / everything before the cursor |
| `Ctrl-D` | Delete a character, or abort when the query is empty |
| `Alt-B` / `Alt-F` | Move a word |
| `Ctrl-L` | Redraw |

## Examples

```sh
# Pick a file and open it
vim "$(find . -type f | ./ccfzf)"

# Preview as you move
find . -type f | ./ccfzf --preview 'head -50 {}' --preview-window=right,60%

# Field matching: a process list keyed on the command, not the PID
ps -ef | ./ccfzf -n 8.. -m | awk '{print $2}'

# Pick a git branch, with a diff preview in a small window
git branch --all | ./ccfzf --height=40% --reverse --preview 'git log --oneline -20 {-1}'

# Non-interactive: use it as a ranking filter in a script
find . -type f | ./ccfzf -f 'algo go$'

# Accept with a key that tells the script what to do next
out=$(find . -type f | ./ccfzf --expect=ctrl-e,ctrl-o --print-query)
```

## Environment

| Variable | Meaning |
|----------|---------|
| `FZF_DEFAULT_COMMAND` | Command that produces the input when nothing is piped in. |
| `FZF_DEFAULT_OPTS` | Options parsed before the command line, so the command line wins. |

With neither set and no pipe, `ccfzf` walks the current directory itself,
skipping hidden directories.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Something was selected |
| 1 | Nothing matched |
| 2 | Usage or runtime error |
| 130 | Interrupted |

## Tests

```sh
go test ./...
```

The matching algorithm is checked against fzf's own published score vectors,
and the interactive loop is driven by tests against a small terminal emulator
— keys go in, a character grid comes out.
