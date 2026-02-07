// Command jsonschema-validate checks a JSON document against a JSON Schema
// (a Draft 7 subset) and lists every error with a JSON Pointer to where it is.
//
// Usage:
//
//	jsonschema-validate [flags] --schema <schema.json> <instance.json>
//
// Exit codes: 0 valid, 1 invalid, 2 usage error, 3 input error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"jsonschema/internal/validator"
)

const (
	exitValid   = 0
	exitInvalid = 1
	exitUsage   = 2
	exitInput   = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}

func run(args []string, stdout, stderr io.Writer, stdin io.Reader) int {
	fs := flag.NewFlagSet("jsonschema-validate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		schemaPath   = fs.String("schema", "", "path to the JSON Schema document (required)")
		format       = fs.String("format", "human", "output format: human|json")
		assertFormat = fs.Bool("assert-format", false, "treat 'format' as an assertion")
		strict       = fs.Bool("strict", false, "warn on unknown keywords / unresolved $ref")
		maxErrors    = fs.Int("max-errors", 0, "stop after N errors (0 = unlimited)")
		maxDepth     = fs.Int("max-depth", validator.DefaultMaxDepth, "recursion / $ref depth guard")
	)

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: jsonschema-validate [flags] --schema <schema.json> <instance.json>\n\n")
		fmt.Fprintf(stderr, "Validates a JSON instance against a JSON Schema (Draft 7 subset).\n")
		fmt.Fprintf(stderr, "Use '-' as the instance path to read from stdin.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	if *schemaPath == "" {
		fmt.Fprintln(stderr, "error: --schema is required")
		fs.Usage()
		return exitUsage
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "error: exactly one instance path is required")
		fs.Usage()
		return exitUsage
	}
	instancePath := rest[0]

	if *format != "human" && *format != "json" {
		fmt.Fprintf(stderr, "error: invalid --format %q (want human|json)\n", *format)
		return exitUsage
	}
	if *maxErrors < 0 {
		fmt.Fprintln(stderr, "error: --max-errors must be >= 0")
		return exitUsage
	}

	schemaBytes, err := os.ReadFile(*schemaPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot read schema: %v\n", err)
		return exitInput
	}
	schema, err := validator.ParseJSON(schemaBytes)
	if err != nil {
		fmt.Fprintf(stderr, "error: schema is not valid JSON: %v\n", err)
		return exitInput
	}

	instanceBytes, err := readInstance(instancePath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot read instance: %v\n", err)
		return exitInput
	}
	instance, err := validator.ParseJSON(instanceBytes)
	if err != nil {
		fmt.Fprintf(stderr, "error: instance is not valid JSON: %v\n", err)
		return exitInput
	}

	v := validator.New(schema, validator.Options{
		AssertFormat: *assertFormat,
		Strict:       *strict,
		MaxDepth:     *maxDepth,
	})
	result := v.Validate(instance)

	errs := result.Errors
	if *maxErrors > 0 && len(errs) > *maxErrors {
		errs = errs[:*maxErrors]
	}

	if *strict {
		for _, w := range result.Warnings {
			fmt.Fprintf(stderr, "warning: %s\n", w)
		}
	}

	if *format == "json" {
		writeJSON(stdout, result.Valid, errs)
	} else {
		writeHuman(stdout, result.Valid, errs)
	}

	if result.Valid {
		return exitValid
	}
	return exitInvalid
}

func readInstance(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func writeHuman(w io.Writer, valid bool, errs []validator.ValidationError) {
	if valid {
		fmt.Fprintln(w, "OK")
		return
	}
	for _, e := range errs {
		loc := e.InstancePath
		if loc == "" {
			loc = "(root)"
		}
		fmt.Fprintf(w, "%s: %s (keyword: %s, schema: %s)\n",
			loc, e.Message, e.Keyword, schemaLoc(e.SchemaPath))
	}
}

func schemaLoc(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

type jsonOutput struct {
	Valid  bool                        `json:"valid"`
	Errors []validator.ValidationError `json:"errors"`
}

func writeJSON(w io.Writer, valid bool, errs []validator.ValidationError) {
	if errs == nil {
		errs = []validator.ValidationError{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(jsonOutput{Valid: valid, Errors: errs})
}
