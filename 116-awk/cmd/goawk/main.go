// Command goawk is a POSIX-subset awk implementation.
package main

import (
	"fmt"
	"os"
	"strings"

	"goawk/interp"
	"goawk/parser"
)

const usage = `usage: goawk [-F fs] [-v var=value]... [-f progfile]... ['program'] [file | var=val]...`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var (
		fsFlag    string
		fsSet     bool
		assigns   []string
		progFiles []string
		progText  string
		haveProg  bool
	)

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}
		switch {
		case arg == "-F":
			i++
			if i >= len(args) {
				return usageErr("-F requires an argument")
			}
			fsFlag, fsSet = args[i], true
		case strings.HasPrefix(arg, "-F"):
			fsFlag, fsSet = arg[2:], true
		case arg == "-v":
			i++
			if i >= len(args) {
				return usageErr("-v requires an argument")
			}
			assigns = append(assigns, args[i])
		case strings.HasPrefix(arg, "-v"):
			assigns = append(assigns, arg[2:])
		case arg == "-f":
			i++
			if i >= len(args) {
				return usageErr("-f requires an argument")
			}
			progFiles = append(progFiles, args[i])
		case strings.HasPrefix(arg, "-f"):
			progFiles = append(progFiles, arg[2:])
		default:
			return usageErr("unknown option " + arg)
		}
		i++
	}

	if len(progFiles) > 0 {
		var sb strings.Builder
		for _, pf := range progFiles {
			data, err := os.ReadFile(pf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "goawk: can't open program file %s\n", pf)
				return 2
			}
			sb.Write(data)
			sb.WriteByte('\n')
		}
		progText = sb.String()
		haveProg = true
	} else {
		if i >= len(args) {
			return usageErr("no program text")
		}
		progText = args[i]
		i++
		haveProg = true
	}
	if !haveProg {
		return usageErr("no program")
	}

	prog, err := parser.Parse(progText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goawk: %s\n", err)
		return 2
	}

	in := interp.New(prog)
	if fsSet {
		in.SetVarString("FS", unescapeFS(fsFlag))
	}
	for _, a := range assigns {
		name, val, ok := splitAssign(a)
		if !ok {
			fmt.Fprintf(os.Stderr, "goawk: invalid -v assignment %q\n", a)
			return 2
		}
		in.SetVarString(name, unescapeFlagValue(val))
	}

	rest := args[i:]
	return in.Run(rest, os.Stdin, os.Stdout, os.Stderr)
}

func usageErr(msg string) int {
	fmt.Fprintf(os.Stderr, "goawk: %s\n%s\n", msg, usage)
	return 2
}

func splitAssign(s string) (name, val string, ok bool) {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return "", "", false
	}
	return s[:eq], s[eq+1:], true
}

// unescapeFS processes backslash escapes in the -F value.
func unescapeFS(fs string) string {
	if fs == "t" {
		// -Ft means a literal "t" here; only -F'\t' is a tab.
		return fs
	}
	return unescapeFlagValue(fs)
}

func unescapeFlagValue(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '/':
			b.WriteByte('/')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
