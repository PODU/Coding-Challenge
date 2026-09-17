// Package exec runs a benchmarked command and measures how long it took.
package exec

import (
	"fmt"
	"runtime"
	"strings"
)

// Shell describes how a command string is handed to an interpreter. A nil or
// empty Argv means "no shell": the command is split into words and executed
// directly.
type Shell struct {
	Argv []string // e.g. ["/bin/sh", "-c"] or ["cmd.exe", "/s", "/c"]
	Name string   // human-readable label used in messages
}

// NoShell is the sentinel spelled `--shell=none` / `-N` on the command line.
var NoShell = Shell{}

// Enabled reports whether this Shell actually wraps commands.
func (s Shell) Enabled() bool { return len(s.Argv) > 0 }

// DefaultShell returns the platform's standard command interpreter.
func DefaultShell() Shell {
	if runtime.GOOS == "windows" {
		return Shell{Argv: []string{"cmd.exe", "/s", "/c"}, Name: "cmd.exe"}
	}
	return Shell{Argv: []string{"/bin/sh", "-c"}, Name: "/bin/sh"}
}

// ParseShell turns the value of `--shell` into a Shell. The value is split with
// shell-like quoting rules so that `--shell="zsh -f"` works, and the flag that
// makes the interpreter read a command string is appended automatically.
func ParseShell(spec string) (Shell, error) {
	if spec == "none" {
		return NoShell, nil
	}
	words, err := Split(spec)
	if err != nil {
		return Shell{}, fmt.Errorf("--shell: %w", err)
	}
	if len(words) == 0 {
		return Shell{}, fmt.Errorf("--shell: empty shell specification")
	}
	argv := append([]string(nil), words...)
	base := strings.ToLower(words[0])
	if strings.Contains(base, "cmd.exe") || base == "cmd" {
		argv = append(argv, "/s", "/c")
	} else {
		argv = append(argv, "-c")
	}
	return Shell{Argv: argv, Name: words[0]}, nil
}

const backslash = '\\'

// Split performs POSIX-ish word splitting: whitespace separates words, and
// single or double quotes protect whitespace inside them. It is used both for
// `--shell` values and for `--shell=none` command lines. On Windows the
// backslash is a path separator, so it is never treated as an escape there.
func Split(s string) ([]string, error) {
	var (
		words   []string
		cur     strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	escapes := runtime.GOOS != "windows"
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			inWord = true
			escaped = false
		case quote != 0:
			switch {
			case r == quote:
				quote = 0
			case r == backslash && quote == '"' && escapes:
				escaped = true
			default:
				cur.WriteRune(r)
			}
			inWord = true
		case r == '\'' || r == '"':
			quote = r
			inWord = true
		case r == backslash && escapes:
			escaped = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote in %q", quote, s)
	}
	if escaped {
		return nil, fmt.Errorf("trailing backslash in %q", s)
	}
	flush()
	return words, nil
}
