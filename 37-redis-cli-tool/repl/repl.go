// Package repl reads commands line by line, either typed at a prompt or piped
// in, and prints each reply. quit, exit and help are handled locally.
package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"rediscli/format"
	"rediscli/help"
	"rediscli/resp"
)

// Sender is the part of *client.Client the REPL uses.
type Sender interface {
	Send(args []string) (resp.Value, error)
	Addr() string
}

const maxLine = 64 * 1024 * 1024 // piped input can have very long lines

// Run reads commands from in and returns the exit status. Interactively it
// shows a prompt and carries on after error replies; when piped it stops at
// the first one and returns 1. Losing the connection always returns 1.
func Run(in io.Reader, out io.Writer, c Sender, interactive bool) int {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)

	for {
		if interactive {
			fmt.Fprintf(out, "%s> ", c.Addr())
		}
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(strings.TrimRight(scanner.Text(), "\r"))
		if line == "" {
			continue
		}

		args, err := tokenize(line)
		if err != nil {
			fmt.Fprintf(out, "%v\n", err)
			if !interactive {
				return 1
			}
			continue
		}
		if len(args) == 0 {
			continue
		}

		switch strings.ToLower(args[0]) {
		case "quit", "exit":
			return 0
		case "help":
			fmt.Fprintln(out, renderHelp(args[1:]))
			continue
		case "clear":
			if interactive {
				continue
			}
		}

		v, err := c.Send(args)
		if err != nil {
			fmt.Fprintf(out, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, format.Render(v))
		if v.Type == resp.Error && !interactive {
			return 1
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(out, "Error: %v\n", err)
		return 1
	}
	return 0
}

func renderHelp(args []string) string {
	if len(args) == 0 {
		return help.Overview()
	}
	target := args[0]
	if strings.HasPrefix(target, "@") {
		if s, ok := help.GroupHelp(target); ok {
			return s
		}
		return fmt.Sprintf("Unknown group: %s", target)
	}
	if s, ok := help.CommandHelp(target); ok {
		return s
	}
	return fmt.Sprintf("Unknown command or group: %s", target)
}

// tokenize splits a line into arguments like redis-cli does. Double quotes
// allow C-style escapes including \xHH; inside single quotes only \' is
// special.
func tokenize(line string) ([]string, error) {
	var args []string
	i, n := 0, len(line)

	for {
		for i < n && isSpace(line[i]) {
			i++
		}
		if i >= n {
			break
		}

		var cur strings.Builder
		inDouble, inSingle := false, false

		for i < n {
			c := line[i]
			switch {
			case inDouble:
				if c == '\\' && i+1 < n {
					esc, adv := unescapeDouble(line[i+1:])
					cur.WriteString(esc)
					i += 1 + adv
					continue
				}
				if c == '"' {
					inDouble = false
					i++
					continue
				}
				cur.WriteByte(c)
				i++
			case inSingle:
				if c == '\\' && i+1 < n && line[i+1] == '\'' {
					cur.WriteByte('\'')
					i += 2
					continue
				}
				if c == '\'' {
					inSingle = false
					i++
					continue
				}
				cur.WriteByte(c)
				i++
			default:
				if isSpace(c) {
					goto done
				}
				switch c {
				case '"':
					inDouble = true
					i++
				case '\'':
					inSingle = true
					i++
				default:
					cur.WriteByte(c)
					i++
				}
			}
		}
	done:
		if inDouble || inSingle {
			return nil, errors.New("Invalid argument(s): unbalanced quotes in request")
		}
		args = append(args, cur.String())
	}
	return args, nil
}

// unescapeDouble decodes the escape that starts just after a backslash and
// returns it with the number of bytes it used.
func unescapeDouble(rest string) (string, int) {
	switch rest[0] {
	case 'n':
		return "\n", 1
	case 'r':
		return "\r", 1
	case 't':
		return "\t", 1
	case 'b':
		return "\b", 1
	case 'a':
		return "\a", 1
	case 'x':
		if len(rest) >= 3 && isHex(rest[1]) && isHex(rest[2]) {
			return string([]byte{hexVal(rest[1])<<4 | hexVal(rest[2])}), 3
		}
		return "x", 1
	default:
		return string(rest[0]), 1
	}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
