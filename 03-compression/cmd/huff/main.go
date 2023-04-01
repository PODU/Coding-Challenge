// Command huff is a Huffman compression / decompression CLI.
//
//	huff compress   [-o out] [-v] [-f] <input>
//	huff decompress [-o out] [-v] [-f] <input>
//	huff -h | --help
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codingchallenges/huff/internal/huffman"
)

const (
	exitOK      = 0
	exitUsage   = 1
	exitIO      = 2
	exitCorrupt = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}

	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return exitOK
	case "compress", "decompress":
	default:
		fmt.Fprintf(stderr, "huff: unknown command %q\n\n", args[0])
		usage(stderr)
		return exitUsage
	}

	cmd := args[0]
	opts, input, err := parseFlags(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "huff: %v\n\n", err)
		usage(stderr)
		return exitUsage
	}
	if input == "" {
		fmt.Fprintln(stderr, "huff: missing input file")
		usage(stderr)
		return exitUsage
	}

	output := opts.output
	if output == "" {
		output = defaultOutput(cmd, input)
	}

	if !opts.force {
		if _, err := os.Stat(output); err == nil {
			fmt.Fprintf(stderr, "huff: output %q already exists (use -f to overwrite)\n", output)
			return exitIO
		}
	}

	switch cmd {
	case "compress":
		return doCompress(input, output, opts, stderr)
	default:
		return doDecompress(input, output, opts, stderr)
	}
}

type options struct {
	output  string
	verbose bool
	force   bool
}

func parseFlags(args []string) (options, string, error) {
	var opts options
	var input string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				return opts, "", errors.New("flag -o requires a value")
			}
			i++
			opts.output = args[i]
		case strings.HasPrefix(a, "-o="):
			opts.output = a[len("-o="):]
		case strings.HasPrefix(a, "--output="):
			opts.output = a[len("--output="):]
		case a == "-v" || a == "--verbose":
			opts.verbose = true
		case a == "-f" || a == "--force":
			opts.force = true
		case a == "--":
			if i+1 < len(args) {
				input = args[i+1]
			}
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			return opts, "", fmt.Errorf("unknown flag %q", a)
		default:
			if input != "" {
				return opts, "", fmt.Errorf("unexpected extra argument %q", a)
			}
			input = a
		}
	}
	return opts, input, nil
}

func defaultOutput(cmd, input string) string {
	if cmd == "compress" {
		return input + ".huff"
	}
	if strings.HasSuffix(input, ".huff") {
		return strings.TrimSuffix(input, ".huff")
	}
	return input + ".out"
}

func doCompress(input, output string, opts options, stderr io.Writer) int {
	in, err := os.Open(input)
	if err != nil {
		fmt.Fprintf(stderr, "huff: cannot open input: %v\n", err)
		return exitIO
	}
	defer in.Close()

	tmp, finalize, err := createTemp(output)
	if err != nil {
		fmt.Fprintf(stderr, "huff: cannot create output: %v\n", err)
		return exitIO
	}
	defer finalize(false)

	stats, err := huffman.Compress(in, tmp)
	if err != nil {
		fmt.Fprintf(stderr, "huff: compress failed: %v\n", err)
		return exitIO
	}
	if err := finalize(true); err != nil {
		fmt.Fprintf(stderr, "huff: cannot write output: %v\n", err)
		return exitIO
	}

	if opts.verbose {
		fmt.Fprintf(stderr, "compressed %s -> %s bytes (%.1f%%, ratio %.2fx), %d distinct symbols\n",
			commas(stats.InputBytes), commas(stats.OutputBytes), stats.Percent(), stats.Ratio(), stats.Distinct)
	}
	return exitOK
}

func doDecompress(input, output string, opts options, stderr io.Writer) int {
	in, err := os.Open(input)
	if err != nil {
		fmt.Fprintf(stderr, "huff: cannot open input: %v\n", err)
		return exitIO
	}
	defer in.Close()

	inInfo, _ := in.Stat()

	tmp, finalize, err := createTemp(output)
	if err != nil {
		fmt.Fprintf(stderr, "huff: cannot create output: %v\n", err)
		return exitIO
	}
	defer finalize(false)

	if err := huffman.Decompress(in, tmp); err != nil {
		if errors.Is(err, huffman.ErrBadMagic) || errors.Is(err, huffman.ErrCorrupt) {
			fmt.Fprintf(stderr, "huff: %v\n", err)
			return exitCorrupt
		}
		fmt.Fprintf(stderr, "huff: decompress failed: %v\n", err)
		return exitIO
	}
	if err := finalize(true); err != nil {
		fmt.Fprintf(stderr, "huff: cannot write output: %v\n", err)
		return exitIO
	}

	if opts.verbose {
		outInfo, _ := os.Stat(output)
		var inSize, outSize int64
		if inInfo != nil {
			inSize = inInfo.Size()
		}
		if outInfo != nil {
			outSize = outInfo.Size()
		}
		fmt.Fprintf(stderr, "decompressed %s -> %s bytes\n",
			commas(uint64(inSize)), commas(uint64(outSize)))
	}
	return exitOK
}

// createTemp creates a temp file next to output so a failed run never leaves
// a half-written file behind. finalize(true) renames it into place and
// finalize(false) deletes it; only the first call does anything, so a
// deferred finalize(false) is safe after a successful commit.
func createTemp(output string) (*os.File, func(bool) error, error) {
	f, err := os.CreateTemp(filepath.Dir(output), ".huff-*.tmp")
	if err != nil {
		return nil, nil, err
	}
	done := false
	finalize := func(commit bool) error {
		if done {
			return nil
		}
		done = true
		if !commit {
			f.Close()
			os.Remove(f.Name())
			return nil
		}
		if err := f.Close(); err != nil {
			os.Remove(f.Name())
			return err
		}
		if err := os.Rename(f.Name(), output); err != nil {
			os.Remove(f.Name())
			return err
		}
		return nil
	}
	return f, finalize, nil
}

// commas formats n with thousands separators, e.g. 3290880 -> "3,290,880".
func commas(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func usage(w io.Writer) {
	fmt.Fprint(w, `huff - Huffman compression tool

Usage:
  huff compress   [-o <output>] [-v] [-f] <input>
  huff decompress [-o <output>] [-v] [-f] <input>
  huff -h | --help

Commands:
  compress     Compress <input> (default output: <input>.huff)
  decompress   Decompress <input> (default output: <input> without .huff)

Flags:
  -o, --output <path>   Explicit output path
  -v, --verbose         Report sizes and compression ratio to stderr
  -f, --force           Overwrite the output file if it exists
  -h, --help            Show this help

Exit codes:
  0 success   1 usage error   2 I/O error   3 corrupt compressed input
`)
}
