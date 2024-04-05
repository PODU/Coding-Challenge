// Command xxd prints a hex dump of a file, or turns a dump back into bytes.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"xxd/internal/xxd"
)

const version = "xxd (Coding Challenge #55) 1.0.0"

const usage = `Usage: xxd [options] [infile]

Make a hexdump of infile (or stdin) to stdout, or reverse a dump back to binary.

Options:
  -l, -len   <n>   dump at most <n> octets
  -s, -seek  <n>   start dump at byte offset <n> (decimal or 0x-hex)
  -c, -cols  <n>   <n> bytes per output line (default 16; 30 with -p)
  -g, -group <n>   <n> bytes per hex group (default 2; 4 with -e; 0 = none)
  -e               little-endian dump (reverse bytes within each group)
  -p, -ps          plain/postscript dump: continuous hex, no offset/ASCII
  -r               reverse: read a hex dump, write binary to stdout
  -h, -help        show this help
  -v, -version     show version
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		lenStr, seekStr, colsStr, groupStr string
		littleEndian, plain, reverse       bool
		showHelp, showVersion              bool
		positional                         []string
	)

	// Parsed by hand: xxd accepts both short and long spellings, and the flag
	// package would print its own usage and exit on errors.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			positional = append(positional, arg)
			i++
			continue
		}

		name := strings.TrimLeft(arg, "-")
		var inlineVal string
		hasInline := false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			inlineVal, hasInline = name[eq+1:], true
			name = name[:eq]
		}

		takeVal := func() (string, bool) {
			if hasInline {
				return inlineVal, true
			}
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}

		ok := true
		switch name {
		case "l", "len":
			lenStr, ok = takeVal()
		case "s", "seek":
			seekStr, ok = takeVal()
		case "c", "cols":
			colsStr, ok = takeVal()
		case "g", "group", "groupsize":
			groupStr, ok = takeVal()
		case "e":
			littleEndian = true
		case "p", "ps", "postscript":
			plain = true
		case "r", "revert":
			reverse = true
		case "h", "help":
			showHelp = true
		case "v", "version":
			showVersion = true
		default:
			fmt.Fprintf(stderr, "xxd: unknown option: %s\n", arg)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if !ok {
			fmt.Fprintf(stderr, "xxd: option %s requires a value\n", arg)
			return 2
		}
		i++
	}

	if showHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	cfg := xxd.NewConfig()
	cfg.LittleEndian = littleEndian
	cfg.Plain = plain
	cfg.Reverse = reverse

	if littleEndian && groupStr == "" {
		cfg.Group = 4
	}
	if plain && colsStr == "" {
		cfg.Cols = xxd.DefaultPlainCols
	}

	if colsStr != "" {
		n, err := parseUint(colsStr)
		if err != nil {
			fmt.Fprintf(stderr, "xxd: invalid -c value %q: %v\n", colsStr, err)
			return 2
		}
		cfg.Cols = int(n)
	}
	if groupStr != "" {
		n, err := parseUint(groupStr)
		if err != nil {
			fmt.Fprintf(stderr, "xxd: invalid -g value %q: %v\n", groupStr, err)
			return 2
		}
		cfg.Group = int(n)
	}
	if lenStr != "" {
		n, err := parseUint(lenStr)
		if err != nil {
			fmt.Fprintf(stderr, "xxd: invalid -l value %q: %v\n", lenStr, err)
			return 2
		}
		cfg.Length = n
	}
	if seekStr != "" {
		n, err := parseUint(seekStr)
		if err != nil {
			fmt.Fprintf(stderr, "xxd: invalid -s value %q: %v\n", seekStr, err)
			return 2
		}
		cfg.Seek = n
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "xxd: %v\n", err)
		return 1
	}

	if len(positional) > 1 {
		fmt.Fprintln(stderr, "xxd: too many input files")
		return 2
	}

	in := stdin
	if len(positional) == 1 && positional[0] != "-" {
		f, err := os.Open(positional[0])
		if err != nil {
			fmt.Fprintf(stderr, "xxd: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}

	var err error
	if reverse {
		err = xxd.Reverse(in, stdout, cfg)
	} else {
		err = xxd.Dump(in, stdout, cfg)
	}
	if err != nil {
		fmt.Fprintf(stderr, "xxd: %v\n", err)
		return 1
	}
	return 0
}

// parseUint parses a non-negative decimal or 0x-prefixed hex integer.
func parseUint(s string) (int64, error) {
	s = strings.TrimSpace(s)
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, fmt.Errorf("not a valid number")
	}
	if n < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return n, nil
}
