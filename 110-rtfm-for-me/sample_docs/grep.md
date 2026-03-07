# grep

## NAME

grep - print lines that match patterns

## SYNOPSIS

grep [OPTION...] PATTERNS [FILE...]

## DESCRIPTION

grep searches for PATTERNS in each FILE. PATTERNS is one or more patterns
separated by newline characters, and grep prints each line that matches a
pattern. Typically PATTERNS should be quoted when grep is used in a shell
command.

## OPTIONS

The `-i` flag ignores case distinctions in patterns and input data, so that
characters that differ only in case match each other.

The `-r` flag reads all files under each directory, recursively. This is
equivalent to the `-d recurse` option and follows symbolic links only if they
are on the command line.

The `-v` flag inverts the sense of matching, to select non-matching lines.

The `-n` flag prefixes each line of output with the 1-based line number within
its input file.

The `-c` flag suppresses normal output and instead prints a count of matching
lines for each input file.

The `-w` flag selects only those lines containing matches that form whole words.

## EXIT STATUS

Normally the exit status is 0 if a line is selected, 1 if no lines were
selected, and 2 if an error occurred. However, if the `-q` or `--quiet` option
is used and a line is selected, the exit status is 0 even if an error occurred.

## SEE ALSO

awk(1), sed(1), regex(7)
