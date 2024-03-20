// Command ccspellcheck is a spell checker backed by a Bloom filter.
//
// Build a model from a dictionary word list:
//
//	ccspellcheck -build dict.txt [-o words.bf] [-p 0.01]
//
// Check words against a model (prints those probably misspelled):
//
//	ccspellcheck [-f words.bf] word1 word2 ...
//
// Exit codes in check mode: 0 if every word is probably correct, 1 if one or
// more words are flagged as misspelled, 2 on usage or I/O error.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"ccspellcheck/internal/bloom"
)

const (
	defaultModel = "words.bf"
	defaultP     = 0.01
)

const (
	exitOK      = 0
	exitFlagged = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// The flag package stops at the first positional argument, so options are
// parsed by hand to let them appear between the words being checked.
type config struct {
	buildPath string
	outPath   string
	modelPath string
	p         float64
	words     []string
	help      bool
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		usage(stderr)
		return exitUsage
	}
	if cfg.help {
		usage(stdout)
		return exitOK
	}

	if cfg.buildPath != "" {
		return runBuild(cfg, stdout, stderr)
	}
	return runCheck(cfg, stdout, stderr)
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		outPath:   defaultModel,
		modelPath: defaultModel,
		p:         defaultP,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "--help":
			cfg.help = true
		case "-build", "--build":
			v, err := nextValue(args, &i, a)
			if err != nil {
				return cfg, err
			}
			cfg.buildPath = v
		case "-o", "--output":
			v, err := nextValue(args, &i, a)
			if err != nil {
				return cfg, err
			}
			cfg.outPath = v
		case "-f", "--file":
			v, err := nextValue(args, &i, a)
			if err != nil {
				return cfg, err
			}
			cfg.modelPath = v
		case "-p", "--prob":
			v, err := nextValue(args, &i, a)
			if err != nil {
				return cfg, err
			}
			pv, perr := parseFloat(v)
			if perr != nil {
				return cfg, fmt.Errorf("invalid -p value %q: %v", v, perr)
			}
			cfg.p = pv
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return cfg, fmt.Errorf("unknown flag %q", a)
			}
			cfg.words = append(cfg.words, a)
		}
	}
	return cfg, nil
}

func nextValue(args []string, i *int, flag string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("flag %s requires a value", flag)
	}
	*i++
	return args[*i], nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err != nil {
		return 0, err
	}
	return f, nil
}

func runBuild(cfg config, stdout, stderr io.Writer) int {
	words, err := readDictionary(cfg.buildPath)
	if err != nil {
		fmt.Fprintln(stderr, "error: reading dictionary:", err)
		return exitUsage
	}

	bf, err := bloom.New(len(words), cfg.p)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	for _, w := range words {
		bf.Add(w)
	}

	out, err := os.Create(cfg.outPath)
	if err != nil {
		fmt.Fprintln(stderr, "error: creating output:", err)
		return exitUsage
	}
	n, werr := bf.WriteTo(out)
	if cerr := out.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		fmt.Fprintln(stderr, "error: writing model:", werr)
		return exitUsage
	}

	fmt.Fprintf(stdout, "built %s: n=%d p=%g m=%d bits k=%d (%d bytes)\n",
		cfg.outPath, len(words), cfg.p, bf.M(), bf.K(), n)
	return exitOK
}

func runCheck(cfg config, stdout, stderr io.Writer) int {
	if len(cfg.words) == 0 {
		fmt.Fprintln(stderr, "error: no words to check")
		usage(stderr)
		return exitUsage
	}

	f, err := os.Open(cfg.modelPath)
	if err != nil {
		fmt.Fprintln(stderr, "error: opening model:", err)
		return exitUsage
	}
	defer f.Close()

	var bf bloom.BloomFilter
	if _, err := bf.ReadFrom(f); err != nil {
		fmt.Fprintln(stderr, "error: loading model:", err)
		return exitUsage
	}

	var flagged []string
	for _, w := range cfg.words {
		if !bf.Contains(w) {
			flagged = append(flagged, w)
		}
	}

	if len(flagged) == 0 {
		fmt.Fprintln(stdout, "no misspellings")
		return exitOK
	}
	for _, w := range flagged {
		fmt.Fprintln(stdout, w)
	}
	return exitFlagged
}

// readDictionary reads one word per line, skipping blank lines and a leading
// UTF-8 BOM.
func readDictionary(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var words []string
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			line = strings.TrimPrefix(line, "\ufeff")
			first = false
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		words = append(words, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return words, nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `ccspellcheck — a Bloom-filter spell checker

Usage:
  Build a model from a dictionary:
    ccspellcheck -build <dict.txt> [-o <words.bf>] [-p <0.01>]

  Check words (prints those probably misspelled):
    ccspellcheck [-f <words.bf>] <word> [<word> ...]

Flags:
  -build <path>   dictionary word list (one word per line) -> build mode
  -o <path>       output model path for build mode (default words.bf)
  -f <path>       model file to load in check mode (default words.bf)
  -p <float>      target false-positive probability for build (default 0.01)
  -h, --help      show this help

Exit codes (check mode): 0 all correct, 1 some misspelled, 2 usage/IO error.
`)
}
