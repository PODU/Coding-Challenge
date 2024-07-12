// Command yaml validates YAML and optionally emits it as JSON.
//
// Usage:
//
//	yaml [flags] [file]
//
// With no file (or "-") it reads stdin. Exit codes: 0 valid, 1 invalid YAML,
// 2 usage/IO error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	yaml "yamlcc"
	"yamlcc/internal/emit"
	"yamlcc/internal/value"
)

func newFlagSet(stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("yaml", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: yaml [flags] [file]")
		fmt.Fprintln(stderr, "  Reads stdin when [file] is omitted or \"-\".")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}
	return fs
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type options struct {
	jsonOut  bool
	validate bool
	ndjson   bool
	indent   int
	strict   bool
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet(stderr)
	opt := &options{}
	fs.BoolVar(&opt.jsonOut, "json", false, "parse and print the data structure as JSON")
	fs.BoolVar(&opt.validate, "validate", false, "validate only; print OK or an error (default mode)")
	fs.BoolVar(&opt.ndjson, "ndjson", false, "for multi-document input, print one JSON value per line")
	fs.IntVar(&opt.indent, "indent", 2, "JSON indent width (0 = compact)")
	fs.BoolVar(&opt.strict, "strict", false, "treat tolerated constructs (directives) as errors")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	src, ec := readSource(fs.Args(), stdin, stderr)
	if ec != 0 {
		return ec
	}

	docs, err := yaml.ParseStrict(src, opt.strict)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}

	if opt.jsonOut || opt.ndjson {
		emitJSON(stdout, docs, opt)
		return 0
	}

	fmt.Fprintln(stdout, "OK")
	return 0
}

func readSource(args []string, stdin io.Reader, stderr io.Writer) ([]byte, int) {
	if len(args) == 0 || args[0] == "-" {
		src, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading stdin: %s\n", err)
			return nil, 2
		}
		return src, 0
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return nil, 2
	}
	return src, 0
}

func emitJSON(stdout io.Writer, docs []value.Value, opt *options) {
	if opt.ndjson {
		for _, d := range docs {
			fmt.Fprintf(stdout, "%s\n", emit.Marshal(d, 0))
		}
		return
	}
	switch len(docs) {
	case 0:
		fmt.Fprintln(stdout, "null")
	case 1:
		fmt.Fprintf(stdout, "%s\n", emit.Marshal(docs[0], opt.indent))
	default:
		fmt.Fprintf(stdout, "%s\n", emit.Marshal(value.SeqValue(docs), opt.indent))
	}
}
