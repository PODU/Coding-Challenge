package cli

import (
	"fmt"
	"strconv"
	"strings"

	"cchyperfine/internal/exec"
	"cchyperfine/internal/param"
)

// Parse turns an argument vector (without the program name) into Options.
//
// The hand-written parser exists because several hyperfine flags take more than
// one operand -- `-P VAR MIN MAX` takes three -- which the standard `flag`
// package cannot express, and because commands are positional arguments that
// may themselves start with a dash.
func Parse(args []string) (*Options, error) {
	o := newOptions()
	shellSpec := ""
	noShell := false

	i := 0
	for i < len(args) {
		a := args[i]
		i++

		if a == "--" { // everything after this is a command
			o.Commands = append(o.Commands, args[i:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			o.Commands = append(o.Commands, a)
			continue
		}

		name, inline, hasInline := splitFlag(a)

		// Collect n operands for the current flag, preferring an inline
		// `--flag=value` if one was given.
		take := func(n int) ([]string, error) {
			var vals []string
			if hasInline {
				vals = append(vals, inline)
				hasInline = false
			}
			for len(vals) < n {
				if i >= len(args) {
					return nil, fmt.Errorf("%s: expected %d value(s)", name, n)
				}
				vals = append(vals, args[i])
				i++
			}
			return vals, nil
		}
		one := func() (string, error) {
			v, err := take(1)
			if err != nil {
				return "", err
			}
			return v[0], nil
		}
		num := func() (int, error) {
			v, err := one()
			if err != nil {
				return 0, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("%s: %q is not a non-negative integer", name, v)
			}
			return n, nil
		}

		var err error
		switch name {
		case "-h", "--help":
			o.ShowHelp = true
		case "-V", "--version":
			o.ShowVersion = true

		case "-r", "--runs":
			o.Runs, err = num()
		case "-m", "--min-runs":
			o.MinRuns, err = num()
		case "-M", "--max-runs":
			o.MaxRuns, err = num()
		case "-w", "--warmup":
			o.Warmup, err = num()

		case "-s", "--setup":
			o.Setup, err = one()
		case "-p", "--prepare":
			var v string
			if v, err = one(); err == nil {
				o.Prepare = append(o.Prepare, v)
			}
		case "-C", "--conclude":
			var v string
			if v, err = one(); err == nil {
				o.Conclude = append(o.Conclude, v)
			}
		case "-c", "--cleanup":
			o.Cleanup, err = one()

		case "-S", "--shell":
			shellSpec, err = one()
			o.ShellGiven = true
		case "-N":
			noShell = true
			o.ShellGiven = true

		case "-u", "--time-unit":
			o.TimeUnit, err = one()
		case "-i", "--ignore-failure":
			o.IgnoreFail = true
		case "-n", "--command-name":
			var v string
			if v, err = one(); err == nil {
				o.Names = append(o.Names, v)
			}
		case "--reference":
			o.Reference, err = one()

		case "-P", "--parameter-scan":
			var v []string
			if v, err = take(3); err == nil {
				o.Scans = append(o.Scans, param.Scan{Var: v[0], Min: v[1], Max: v[2]})
			}
		case "-D", "--parameter-step-size":
			o.Step, err = one()
		case "-L", "--parameter-list":
			var v []string
			if v, err = take(2); err == nil {
				o.Lists = append(o.Lists, param.List{Var: v[0], Values: splitList(v[1])})
			}

		case "--export-json":
			o.ExportJSON, err = one()
		case "--export-csv":
			o.ExportCSV, err = one()
		case "--export-markdown":
			o.ExportMarkdown, err = one()
		case "--export-asciidoc":
			o.ExportAsciidoc, err = one()
		case "--export-orgmode":
			o.ExportOrgmode, err = one()

		case "--output":
			var v string
			if v, err = one(); err == nil {
				err = o.setOutput(v)
			}
		case "--show-output":
			o.ShowOutput = true
		case "--input":
			var v string
			if v, err = one(); err == nil {
				if v == "null" {
					o.InputIsNull = true
					o.InputPath = ""
				} else {
					o.InputPath = v
				}
			}
		case "--sort":
			o.SortMethod, err = one()
		case "--style":
			o.Style, err = one()

		default:
			// A cluster of boolean short flags, e.g. `-Ni`.
			if expanded, ok := expandCluster(a); ok {
				args = append(expanded, args[i:]...)
				i = 0
				continue
			}
			return nil, fmt.Errorf("unknown option %q (try --help)", a)
		}
		if err != nil {
			return nil, err
		}
		if hasInline {
			return nil, fmt.Errorf("%s does not take a value", name)
		}
	}

	if noShell {
		o.Shell = exec.NoShell
	} else if shellSpec != "" {
		s, err := exec.ParseShell(shellSpec)
		if err != nil {
			return nil, err
		}
		o.Shell = s
	}
	if o.ShowOutput {
		o.OutputMode = OutputInherit
	}
	if err := o.validate(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Options) setOutput(v string) error {
	switch v {
	case "null":
		o.OutputMode = OutputNull
	case "pipe":
		o.OutputMode = OutputPipe
	case "inherit":
		o.OutputMode = OutputInherit
	default:
		o.OutputMode = OutputFile
		o.OutputFile = v
	}
	return nil
}

// splitFlag separates `--name=value` into its parts. Short flags written as
// `-r5` are also accepted, but only when the character after the letter is not
// itself a flag letter -- that ambiguity is resolved by expandCluster.
func splitFlag(a string) (name, value string, ok bool) {
	if strings.HasPrefix(a, "--") {
		if i := strings.IndexByte(a, '='); i > 0 {
			return a[:i], a[i+1:], true
		}
		return a, "", false
	}
	if len(a) > 2 {
		if a[2] == '=' {
			return a[:2], a[3:], true
		}
		if takesValue(a[:2]) {
			return a[:2], a[2:], true
		}
	}
	return a, "", false
}

func takesValue(short string) bool {
	switch short {
	case "-r", "-m", "-M", "-w", "-s", "-p", "-C", "-c", "-S", "-u", "-n", "-D":
		return true
	}
	return false
}

// expandCluster turns `-Ni` into ["-N", "-i"]. It only succeeds if every
// character maps to a known boolean short flag.
func expandCluster(a string) ([]string, bool) {
	if strings.HasPrefix(a, "--") || len(a) < 3 {
		return nil, false
	}
	var out []string
	for _, r := range a[1:] {
		f := "-" + string(r)
		switch f {
		case "-N", "-i", "-h", "-V":
			out = append(out, f)
		default:
			return nil, false
		}
	}
	return out, true
}

// splitList parses a comma-separated `--parameter-list` value. A comma can be
// included in a value by escaping it as `\,`.
func splitList(s string) []string {
	var (
		out []string
		cur strings.Builder
		esc bool
	)
	for _, r := range s {
		switch {
		case esc:
			if r != ',' {
				cur.WriteRune('\\')
			}
			cur.WriteRune(r)
			esc = false
		case r == '\\':
			esc = true
		case r == ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if esc {
		cur.WriteRune('\\')
	}
	out = append(out, cur.String())
	return out
}
