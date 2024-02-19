// Command passcrack is an educational password cracker: given one or more
// password hashes it recovers the plaintext by hashing candidate passwords and
// comparing digests, via dictionary, brute-force, or rainbow-table strategies.
//
// EDUCATIONAL / AUTHORIZED USE ONLY. See README.md and the banner printed on
// start. Use it only on hashes you own or are explicitly authorized to test.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"passcrack/bruteforce"
	"passcrack/engine"
	"passcrack/hashreg"
	"passcrack/rainbow"
	"passcrack/saltutil"
	"passcrack/wordlist"
)

const (
	exitAllCracked = 0
	exitUncracked  = 1
	exitUsage      = 2
)

const ethicsBanner = `passcrack — educational password cracker.
  AUTHORIZED USE ONLY: crack only hashes you own or are explicitly permitted to
  test. Unauthorized cracking of credentials is illegal in most jurisdictions.
  The lesson: fast, unsalted hashes (MD5/SHA-1) fall in seconds — use a slow,
  salted KDF (bcrypt/scrypt/argon2/PBKDF2) for real password storage.
`

type options struct {
	algo        string
	mode        string
	wordlist    string
	hashesFile  string
	charset     string
	min         int
	max         int
	salt        string
	saltMode    string
	workers     int
	rainbowFile string
	buildRT     bool
	quiet       bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opt, target, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitAllCracked
		}
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	if !opt.quiet {
		fmt.Fprint(stderr, ethicsBanner)
	}

	saltMode, err := saltutil.ParseMode(opt.saltMode)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	salter := saltutil.New(opt.salt, saltMode)

	if opt.mode == "rainbow" && opt.buildRT {
		return runRainbowBuild(opt, salter, stderr)
	}

	targets, err := collectTargets(target, opt.hashesFile)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	switch opt.mode {
	case "rainbow":
		return runRainbowLookup(opt, targets, stdout, stderr)
	case "dict", "brute":
		return runCrack(opt, targets, salter, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: invalid mode %q (want dict|brute|rainbow)\n", opt.mode)
		return exitUsage
	}
}

func parseFlags(args []string, stderr io.Writer) (options, string, error) {
	var opt options
	fs := flag.NewFlagSet("passcrack", flag.ContinueOnError)
	fs.SetOutput(stderr)

	strVar := func(p *string, def string, names ...string) {
		for _, n := range names {
			fs.StringVar(p, n, def, "")
		}
	}
	intVar := func(p *int, def int, names ...string) {
		for _, n := range names {
			fs.IntVar(p, n, def, "")
		}
	}
	boolVar := func(p *bool, def bool, names ...string) {
		for _, n := range names {
			fs.BoolVar(p, n, def, "")
		}
	}

	strVar(&opt.algo, "md5", "a", "algo")
	strVar(&opt.mode, "dict", "m", "mode")
	strVar(&opt.wordlist, "", "w", "wordlist")
	strVar(&opt.hashesFile, "", "hashes-file")
	strVar(&opt.charset, "abcdefghijklmnopqrstuvwxyz0123456789", "charset")
	intVar(&opt.min, 1, "min")
	intVar(&opt.max, 6, "max")
	strVar(&opt.salt, "", "salt")
	strVar(&opt.saltMode, "prefix", "salt-mode")
	intVar(&opt.workers, 0, "workers")
	strVar(&opt.rainbowFile, "", "rainbow-file")
	boolVar(&opt.buildRT, false, "build-rainbow")
	boolVar(&opt.quiet, false, "q", "quiet")

	fs.Usage = func() { printUsage(stderr) }

	if err := fs.Parse(args); err != nil {
		return opt, "", err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return opt, "", fmt.Errorf("at most one positional TARGET_HASH allowed, got %d", len(rest))
	}
	var target string
	if len(rest) == 1 {
		target = rest[0]
	}
	return opt, target, nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `passcrack [flags] [TARGET_HASH]   — educational password cracker (authorized use only)

Flags:
  -a, --algo string      md5 | sha1 | sha256 | sha512 | auto   (default "md5")
  -m, --mode string      dict | brute | rainbow                (default "dict")
  -w, --wordlist string  wordlist path, or "-" for stdin
      --hashes-file str  file of target hashes, one per line
      --charset string   brute-force alphabet                  (default a-z0-9)
      --min int          brute-force min length                (default 1)
      --max int          brute-force max length                (default 6)
      --salt string      optional salt value
      --salt-mode str    prefix | suffix                       (default "prefix")
      --workers int      worker goroutines                     (default NumCPU)
      --rainbow-file s   rainbow table path (build/lookup)
      --build-rainbow    build the table instead of looking up
  -q, --quiet            suppress stderr banner/progress
  -h, --help

Exit codes: 0 = all cracked, 1 = some uncracked, 2 = usage/IO error.
`)
}

// resolveAlgo returns the algorithm named by opt.algo, supporting "auto"
// detection from the digest length of the first target.
func resolveAlgo(name string, targets []string) (hashreg.Algo, error) {
	if strings.EqualFold(strings.TrimSpace(name), "auto") {
		if len(targets) == 0 {
			return nil, errors.New("auto algorithm detection needs at least one target hash")
		}
		hexLen := len(strings.TrimSpace(targets[0]))
		for _, t := range targets[1:] {
			if len(strings.TrimSpace(t)) != hexLen {
				return nil, errors.New("auto detection requires all target hashes to share one length")
			}
		}
		return hashreg.Detect(hexLen)
	}
	return hashreg.Get(name)
}

func collectTargets(positional, hashesFile string) ([]string, error) {
	var targets []string
	if positional != "" {
		targets = append(targets, positional)
	}
	if hashesFile != "" {
		f, err := os.Open(hashesFile)
		if err != nil {
			return nil, fmt.Errorf("opening hashes file: %w", err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			targets = append(targets, line)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("reading hashes file: %w", err)
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("no target hash given (positional TARGET_HASH or --hashes-file)")
	}
	return targets, nil
}

func runRainbowBuild(opt options, salter saltutil.Salter, stderr io.Writer) int {
	if opt.rainbowFile == "" {
		fmt.Fprintln(stderr, "error: rainbow mode requires --rainbow-file")
		return exitUsage
	}
	algo, err := resolveAlgo(opt.algo, nil)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	words, closeFn, err := openWordsource(opt.wordlist)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	defer closeFn()
	out, err := os.Create(opt.rainbowFile)
	if err != nil {
		fmt.Fprintln(stderr, "error: creating rainbow file:", err)
		return exitUsage
	}
	defer out.Close()
	n, err := rainbow.Build(context.Background(), words, out, algo, salter)
	if err != nil {
		fmt.Fprintln(stderr, "error: building rainbow table:", err)
		return exitUsage
	}
	if !opt.quiet {
		fmt.Fprintf(stderr, "built rainbow table %q with %d entries (%s)\n", opt.rainbowFile, n, algo.Name())
	}
	return exitAllCracked
}

func runRainbowLookup(opt options, targets []string, stdout, stderr io.Writer) int {
	if opt.rainbowFile == "" {
		fmt.Fprintln(stderr, "error: rainbow mode requires --rainbow-file")
		return exitUsage
	}
	f, err := os.Open(opt.rainbowFile)
	if err != nil {
		fmt.Fprintln(stderr, "error: opening rainbow file:", err)
		return exitUsage
	}
	defer f.Close()
	cracked, miss, err := rainbow.Lookup(f, targets)
	if err != nil {
		fmt.Fprintln(stderr, "error: reading rainbow file:", err)
		return exitUsage
	}
	for _, t := range normTargets(targets) {
		if pt, ok := cracked[t]; ok {
			fmt.Fprintf(stdout, "%s:%s\n", t, pt)
		}
	}
	return reportUncracked(miss, stderr, opt.quiet)
}

func runCrack(opt options, targets []string, salter saltutil.Salter, stdout, stderr io.Writer) int {
	algo, err := resolveAlgo(opt.algo, targets)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	cr, err := engine.New(engine.Config{
		Algo:    algo,
		Salter:  salter,
		Targets: targets,
		Workers: opt.workers,
	})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var candidates <-chan string
	var producerErr <-chan error
	switch opt.mode {
	case "dict":
		words, closeFn, err := openWordsource(opt.wordlist)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		defer closeFn()
		candidates, producerErr = wordlist.Stream(ctx, words)
	case "brute":
		if err := bruteforce.Validate(opt.charset, opt.min, opt.max); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		candidates = bruteforce.Generate(ctx, opt.charset, opt.min, opt.max)
	}

	if !opt.quiet {
		fmt.Fprintf(stderr, "cracking %d hash(es) with %s, mode=%s, workers=%d\n",
			cr.TargetCount(), algo.Name(), opt.mode, cr.Workers())
	}

	w := bufio.NewWriter(stdout)
	cracked, uncracked := cr.Run(ctx, candidates, func(r engine.Result) {
		fmt.Fprintf(w, "%s:%s\n", r.Hash, r.Plaintext)
		w.Flush()
	})
	w.Flush()

	if producerErr != nil {
		select {
		case err := <-producerErr:
			if err != nil {
				fmt.Fprintln(stderr, "error: reading wordlist:", err)
				return exitUsage
			}
		default:
		}
	}

	if !opt.quiet {
		fmt.Fprintf(stderr, "cracked %d/%d\n", len(cracked), cr.TargetCount())
	}
	return reportUncracked(uncracked, stderr, opt.quiet)
}

// openWordsource opens the wordlist path, or stdin for "-".
func openWordsource(path string) (io.Reader, func(), error) {
	if path == "" {
		return nil, func() {}, errors.New("dictionary/rainbow-build mode requires -w WORDLIST (or -w - for stdin)")
	}
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening wordlist: %w", err)
	}
	return f, func() { f.Close() }, nil
}

func reportUncracked(uncracked []string, stderr io.Writer, quiet bool) int {
	if len(uncracked) == 0 {
		return exitAllCracked
	}
	if !quiet {
		for _, h := range uncracked {
			fmt.Fprintf(stderr, "UNCRACKED %s\n", h)
		}
	}
	return exitUncracked
}

func normTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	seen := make(map[string]struct{})
	for _, t := range targets {
		n := strings.ToLower(strings.TrimSpace(t))
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}
