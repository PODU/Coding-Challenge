# ccsed — a `sed` clone (Coding Challenge #21)

`ccsed` is a stream editor in the spirit of Unix `sed`. It reads text line by
line (the *pattern space*), applies an ordered list of editing commands — each
optionally guarded by an *address* — and writes the result to stdout or back to
the file with `-i`.

Regex matching is delegated to Go's `regexp` package (RE2), so matching is
linear-time with no catastrophic backtracking. POSIX **Basic Regular
Expressions** (sed's default dialect) are translated to RE2 before compilation.

## Build

```sh
cd 21-sed
go build -o ccsed ./...      # ccsed.exe on Windows
```

Zero third-party dependencies; Go 1.24+.

## Usage

```
ccsed [OPTIONS] {script | -e script... | -f file...} [input-file...]
```

| Option       | Meaning                                                        |
|--------------|----------------------------------------------------------------|
| `-n`         | Quiet: suppress automatic printing of the pattern space.       |
| `-e SCRIPT`  | Add `SCRIPT` to the program (repeatable).                      |
| `-f FILE`    | Add the contents of `FILE` to the program (repeatable).        |
| `-i[SUFFIX]` | Edit files in place; if `SUFFIX` is given, back up to `NAME+SUFFIX`. |
| `--`         | Treat all following arguments as operands.                     |
| `-h`, `--help` | Print help and exit.                                         |

With no `-e`/`-f`, the first operand is the script and the rest are input files.
With no input files, `ccsed` reads stdin (use `-` to read stdin explicitly).
Multiple `-e`/`-f` scripts are joined with newlines.

### Commands

| Command            | Effect                                                       |
|--------------------|--------------------------------------------------------------|
| `s/re/repl/flags`  | Substitute. Flags: `g` (all), `p` (print), `N` (Nth match), `Ng` (from Nth on), `i`/`I` (ignore case), `m`/`M` (multiline). |
| `p`                | Print the pattern space.                                     |
| `d`                | Delete the pattern space and end the cycle.                  |
| `G`                | Append a blank line (double-spacing).                        |

The `s` delimiter may be any character (`s|a|b|`, `s#a#b#`); escape it inside the
pattern with a backslash (`s/a\/b/x/`). Replacements support `&` (whole match),
`\1`–`\9` (capture groups), and `\n`/`\t`.

### Addresses

| Address      | Selects                                              |
|--------------|------------------------------------------------------|
| `N`          | line number `N`                                      |
| `$`          | the last line                                        |
| `/re/`       | lines matching `re` (also `\cREc` for a custom delim) |
| `A,B`        | the range from `A` to `B` (numeric or regex bounds)  |
| `A,B!cmd`    | negation — apply `cmd` to lines **not** selected      |
| *(none)*     | every line                                           |

Regex ranges re-trigger: after `/a/,/b/` closes, a later `/a/` opens it again.

Separate commands with `;` or newlines. `#` begins a comment; a leading `#n`
line implies `-n`.

## Examples

```sh
ccsed 's/"//g' test.txt              # strip every double quote
cat -n file | ccsed -n '2,4p'        # print only lines 2-4
ccsed -n '/roads/p' file             # grep-like: lines containing "roads"
ccsed 'G' file                       # double-space the file
ccsed '/^$/d' file                   # delete blank lines
ccsed -i 's/Life/Code/g' file        # edit in place
ccsed -i.bak 's/Life/Code/g' file    # edit in place, keep file.bak
ccsed -n -e '1p' -e '3p' file        # multiple scripts
```

## Exit status

| Code | Meaning                                       |
|------|-----------------------------------------------|
| `0`  | success                                        |
| `1`  | runtime error (missing input file, write failure) |
| `2`  | usage or script parse error                    |

## Notes and caveats

- **Line endings.** CRLF input is normalized: a trailing `\r` is stripped before
  matching and output uses `\n`. The presence/absence of a trailing newline on
  the final line is preserved.
- **Empty regex `//`** (reuse of the last regex) is rejected with a clear error
  rather than silently reused.
- **Multiple input files** are concatenated into one stream, so ranges and `$`
  span the whole input (GNU `sed`'s default, non-`-s`, behaviour). `-i` resets
  range state per file.
- **BRE translation** covers the common cases: `\( \) \{ \} \+ \? \|` become
  operators, their bare forms are literal, and bracket expressions are copied
  verbatim. Backreferences *inside a pattern* are not supported (an RE2 limit).
- **Out of scope** (rejected with an error, not silently ignored): hold-space
  commands beyond `G` (`h H g x`), branching/labels (`b t :`), `a`/`i`/`c`,
  `y`, `w`/`r`, `l`, `=`, `q`, and GNU `\L`/`\U`.

## Tests

```sh
go test ./...                                  # unit + golden + CLI + in-place
go test -run x -fuzz FuzzParse -fuzztime 10s   # parser fuzzing (never panics)
```

The suite covers substitution flags (`g`/`p`/`N`/`Ng`), backreferences and `&`,
all address forms, numeric/regex ranges and re-triggering, negation, `-n`,
multiple `-e`, alternate/escaped delimiters, trailing-newline preservation,
zero-width-match safety, in-place editing with and without backups, and the
`run()` entry point's exit codes.
