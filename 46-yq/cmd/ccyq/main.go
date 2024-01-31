// Command ccyq is a jq-like processor for YAML (and JSON) documents.
//
// Usage:
//
//	ccyq [options] '<filter>' [file]
//
// See README.md for the supported filter subset and flags.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"ccyq/query"
	"ccyq/yamlio"

	"gopkg.in/yaml.v3"
)

const version = "ccyq 1.0.0"

const usage = `ccyq — a jq for YAML (Coding Challenge #46)

Usage:
  ccyq [options] '<filter>' [file]

  <filter>   path/filter expression (default '.', the identity filter)
  file       input file; if omitted, read from stdin

Options:
  -o, --output <yaml|json>   output format            (default yaml)
  -p, --input  <yaml|json>   input format             (default yaml)
  -r, --raw                  raw output for string scalars (no quotes)
  -c, --compact              compact JSON (one doc per line)
  -i, --inplace              edit the input file in place (requires file)
      --no-doc-sep           omit '---' between multiple output YAML docs
  -h, --help                 show this help
  -v, --version              show version
`

const (
	exitOK    = 0
	exitEval  = 1
	exitUsage = 2 // bad flags or filter syntax
	exitIO    = 3
)

type config struct {
	filter   string
	file     string
	hasFile  bool
	inFormat string
	outFmt   string
	raw      bool
	compact  bool
	inplace  bool
	noDocSep bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, code, msg, done := parseArgs(args)
	if done {
		if msg != "" {
			fmt.Fprint(stderr, msg)
		}
		return code
	}

	filter, err := query.Compile(cfg.filter)
	if err != nil {
		fmt.Fprintf(stderr, "ccyq: %v\n", err)
		return exitUsage
	}

	var data []byte
	if cfg.hasFile {
		data, err = os.ReadFile(cfg.file)
		if err != nil {
			fmt.Fprintf(stderr, "ccyq: %v\n", err)
			return exitIO
		}
	} else {
		data, err = io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "ccyq: reading stdin: %v\n", err)
			return exitIO
		}
	}

	var docs []*yaml.Node
	if cfg.inFormat == "json" {
		docs, err = yamlio.LoadJSON(data)
	} else {
		docs, err = yamlio.LoadYAML(data)
	}
	if err != nil {
		fmt.Fprintf(stderr, "ccyq: %v\n", err)
		return exitEval
	}

	var results []*yaml.Node
	for _, doc := range docs {
		out, err := filter.Eval(doc)
		if err != nil {
			fmt.Fprintf(stderr, "ccyq: %v\n", err)
			return exitEval
		}
		results = append(results, out...)
	}

	opts := yamlio.Options{
		Format:   cfg.outFmt,
		Raw:      cfg.raw,
		Compact:  cfg.compact,
		NoDocSep: cfg.noDocSep,
	}

	// Render into a buffer first so a failure part-way never leaves a
	// half-written file or partial stdout.
	var buf bytes.Buffer
	if err := yamlio.Emit(&buf, results, opts); err != nil {
		fmt.Fprintf(stderr, "ccyq: %v\n", err)
		return exitEval
	}

	if cfg.inplace {
		if err := os.WriteFile(cfg.file, buf.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "ccyq: writing %s: %v\n", cfg.file, err)
			return exitIO
		}
		return exitOK
	}

	if _, err := stdout.Write(buf.Bytes()); err != nil {
		fmt.Fprintf(stderr, "ccyq: %v\n", err)
		return exitIO
	}
	return exitOK
}

// parseArgs parses the command line. When done is true the caller should print
// msg (if any) and exit with code.
func parseArgs(args []string) (cfg config, code int, msg string, done bool) {
	cfg = config{inFormat: "yaml", outFmt: "yaml"}
	var positionals []string

	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "-h", "--help":
			return cfg, exitOK, usage, true
		case "-v", "--version":
			return cfg, exitOK, version + "\n", true
		case "-r", "--raw":
			cfg.raw = true
		case "-c", "--compact":
			cfg.compact = true
		case "-i", "--inplace":
			cfg.inplace = true
		case "--no-doc-sep":
			cfg.noDocSep = true
		case "-o", "--output":
			i++
			if i >= len(args) {
				return cfg, exitUsage, "ccyq: " + a + " requires a value (yaml|json)\n", true
			}
			cfg.outFmt = args[i]
		case "-p", "--input":
			i++
			if i >= len(args) {
				return cfg, exitUsage, "ccyq: " + a + " requires a value (yaml|json)\n", true
			}
			cfg.inFormat = args[i]
		default:
			if len(a) > 1 && a[0] == '-' && a != "-" {
				// --output=json / -o=json
				if k, v, ok := strings.Cut(a, "="); ok {
					switch k {
					case "-o", "--output":
						cfg.outFmt = v
					case "-p", "--input":
						cfg.inFormat = v
					default:
						return cfg, exitUsage, "ccyq: unknown option " + a + "\n", true
					}
				} else {
					return cfg, exitUsage, "ccyq: unknown option " + a + "\n", true
				}
			} else {
				positionals = append(positionals, a)
			}
		}
		i++
	}

	if cfg.outFmt != "yaml" && cfg.outFmt != "json" {
		return cfg, exitUsage, "ccyq: invalid output format " + cfg.outFmt + " (want yaml|json)\n", true
	}
	if cfg.inFormat != "yaml" && cfg.inFormat != "json" {
		return cfg, exitUsage, "ccyq: invalid input format " + cfg.inFormat + " (want yaml|json)\n", true
	}

	switch len(positionals) {
	case 0:
		cfg.filter = "."
	case 1:
		cfg.filter = positionals[0]
	case 2:
		cfg.filter = positionals[0]
		cfg.file = positionals[1]
		cfg.hasFile = true
	default:
		return cfg, exitUsage, "ccyq: too many arguments\n", true
	}

	if cfg.inplace && !cfg.hasFile {
		return cfg, exitUsage, "ccyq: -i/--inplace requires a file argument\n", true
	}
	return cfg, exitOK, "", false
}
