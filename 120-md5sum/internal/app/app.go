// Package app is the md5sum command line, kept out of package main so it can
// be tested directly.
package app

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"

	"md5sum/internal/md5"
	"md5sum/internal/sha256"
)

const version = "md5sum (coding-challenge-120) 1.0.0"

const usage = `Usage: md5sum [OPTION]... [FILE]...
Print or check MD5 (128-bit) checksums.

With no FILE, or when FILE is "-", read standard input.

  -c, --check          read checksums from the FILEs and verify them
  -b, --binary         read in binary mode (output "*" before filename)
  -t, --text           read in text mode (default)
      --tag            create a BSD-style (reversed) checksum line
      --quiet          (with -c) don't print OK for each verified file
      --status         (with -c) silent; signal result via exit code only
  -a, --algorithm A    digest algorithm: md5 (default) or sha256
      --help           display this help and exit
      --version        output version information and exit
`

// algorithm is everything that differs between MD5 and SHA-256.
type algorithm struct {
	name   string
	tag    string // label used in BSD-style (--tag) lines
	hexLen int
	newFn  func() hash.Hash
}

func md5Algo() algorithm {
	return algorithm{name: "md5", tag: "MD5", hexLen: md5.Size * 2, newFn: func() hash.Hash { return md5.New() }}
}

func sha256Algo() algorithm {
	return algorithm{name: "sha256", tag: "SHA256", hexLen: sha256.Size * 2, newFn: func() hash.Hash { return sha256.New() }}
}

type options struct {
	check    bool
	binary   bool
	tag      bool
	quiet    bool
	status   bool
	algo     algorithm
	files    []string
	showHelp bool
	showVers bool
}

// Run runs md5sum with args (without the program name) and returns the exit
// code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "md5sum: %v\n", err)
		fmt.Fprint(stderr, "Try 'md5sum --help' for more information.\n")
		return 2
	}
	if opts.showHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if opts.showVers {
		fmt.Fprintln(stdout, version)
		return 0
	}

	if opts.check {
		return runCheck(opts, stdin, stdout, stderr)
	}
	return runHash(opts, stdin, stdout, stderr)
}

// parseArgs handles GNU-style options: long options, clustered short options
// such as -bc, "--", and "-" for stdin.
func parseArgs(args []string) (options, error) {
	opts := options{algo: md5Algo()}
	algoName := "md5"
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case endOfFlags:
			opts.files = append(opts.files, arg)
		case arg == "--":
			endOfFlags = true
		case arg == "-":
			opts.files = append(opts.files, arg)
		case strings.HasPrefix(arg, "--"):
			name := arg[2:]
			value := ""
			hasValue := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				value = name[eq+1:]
				name = name[:eq]
				hasValue = true
			}
			switch name {
			case "check":
				opts.check = true
			case "binary":
				opts.binary = true
			case "text":
				opts.binary = false
			case "tag":
				opts.tag = true
			case "quiet":
				opts.quiet = true
			case "status":
				opts.status = true
			case "help":
				opts.showHelp = true
			case "version":
				opts.showVers = true
			case "algorithm":
				if !hasValue {
					i++
					if i >= len(args) {
						return opts, fmt.Errorf("option '--algorithm' requires an argument")
					}
					value = args[i]
				}
				algoName = value
			default:
				return opts, fmt.Errorf("unrecognized option '--%s'", name)
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			for j := 1; j < len(arg); j++ {
				c := arg[j]
				switch c {
				case 'c':
					opts.check = true
				case 'b':
					opts.binary = true
				case 't':
					opts.binary = false
				case 'a':
					// The value is the rest of this arg, or else the next arg.
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							return opts, fmt.Errorf("option requires an argument -- 'a'")
						}
						val = args[i]
					}
					algoName = val
					j = len(arg)
				default:
					return opts, fmt.Errorf("invalid option -- '%c'", c)
				}
			}
		default:
			opts.files = append(opts.files, arg)
		}
	}

	switch algoName {
	case "md5", "MD5":
		opts.algo = md5Algo()
	case "sha256", "SHA256":
		opts.algo = sha256Algo()
	default:
		return opts, fmt.Errorf("unknown algorithm %q (supported: md5, sha256)", algoName)
	}

	if opts.tag && opts.check {
		return opts, fmt.Errorf("the --tag option is meaningless when verifying checksums")
	}
	if opts.tag && opts.binary {
		return opts, fmt.Errorf("the --tag option is incompatible with --binary")
	}
	if !opts.check && (opts.quiet || opts.status) {
		return opts, fmt.Errorf("the --quiet and --status options are meaningful only when verifying checksums")
	}

	return opts, nil
}

// hashReader returns the lowercase hex digest of r.
func hashReader(algo algorithm, r io.Reader) (string, error) {
	h := algo.newFn()
	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(h, r, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(algo algorithm, path string, stdin io.Reader) (string, error) {
	if path == "-" {
		return hashReader(algo, stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// Same message coreutils prints for a directory.
	if info, statErr := f.Stat(); statErr == nil && info.IsDir() {
		return "", fmt.Errorf("Is a directory")
	}
	return hashReader(algo, f)
}

func formatLine(opts options, digest, name string) string {
	switch {
	case opts.tag:
		return fmt.Sprintf("%s (%s) = %s\n", opts.algo.tag, name, digest)
	case opts.binary:
		return fmt.Sprintf("%s *%s\n", digest, name)
	default:
		return fmt.Sprintf("%s  %s\n", digest, name)
	}
}

func runHash(opts options, stdin io.Reader, stdout, stderr io.Writer) int {
	files := opts.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	exit := 0
	for _, f := range files {
		digest, err := hashFile(opts.algo, f, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "md5sum: %s: %s\n", f, ioErr(err))
			exit = 1
			continue
		}
		fmt.Fprint(stdout, formatLine(opts, digest, f))
	}
	return exit
}

type checkEntry struct {
	digest string // lowercase hex
	name   string
	binary bool
}

// parseCheckLine accepts both GNU ("<hex>  name" / "<hex> *name") and BSD
// ("MD5 (name) = <hex>") lines. ok is false for blank or malformed lines.
func parseCheckLine(line string, algo algorithm) (entry checkEntry, ok bool) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return checkEntry{}, false
	}

	prefix := algo.tag + " ("
	if strings.HasPrefix(line, prefix) {
		rest := line[len(prefix):]
		close := strings.LastIndex(rest, ") = ")
		if close < 0 {
			return checkEntry{}, false
		}
		name := rest[:close]
		digest := rest[close+len(") = "):]
		if !validHex(digest, algo.hexLen) {
			return checkEntry{}, false
		}
		return checkEntry{digest: strings.ToLower(digest), name: name, binary: true}, true
	}

	if len(line) < algo.hexLen+2 {
		return checkEntry{}, false
	}
	digest := line[:algo.hexLen]
	if !validHex(digest, algo.hexLen) || line[algo.hexLen] != ' ' {
		return checkEntry{}, false
	}
	marker := line[algo.hexLen+1]
	if marker != '*' && marker != ' ' {
		return checkEntry{}, false
	}
	binary := marker == '*'
	name := line[algo.hexLen+2:]
	if name == "" {
		return checkEntry{}, false
	}
	return checkEntry{digest: strings.ToLower(digest), name: name, binary: binary}, true
}

func validHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func runCheck(opts options, stdin io.Reader, stdout, stderr io.Writer) int {
	checkFiles := opts.files
	if len(checkFiles) == 0 {
		checkFiles = []string{"-"}
	}

	exit := 0
	var totalFormatErr, totalReadErr, totalMismatch int

	for _, cf := range checkFiles {
		var reader io.Reader
		if cf == "-" {
			reader = stdin
		} else {
			f, err := os.Open(cf)
			if err != nil {
				fmt.Fprintf(stderr, "md5sum: %s: %s\n", cf, ioErr(err))
				exit = 1
				continue
			}
			defer f.Close()
			reader = f
		}

		sc := bufio.NewScanner(reader)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		var fileFormatErr, fileReadErr, fileMismatch, processed int

		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			entry, ok := parseCheckLine(line, opts.algo)
			if !ok {
				fileFormatErr++
				continue
			}
			processed++

			digest, err := hashFile(opts.algo, entry.name, stdin)
			if err != nil {
				if !opts.status {
					fmt.Fprintf(stdout, "%s: FAILED open or read\n", entry.name)
				}
				fileReadErr++
				continue
			}
			if digest == entry.digest {
				if !opts.quiet && !opts.status {
					fmt.Fprintf(stdout, "%s: OK\n", entry.name)
				}
			} else {
				if !opts.status {
					fmt.Fprintf(stdout, "%s: FAILED\n", entry.name)
				}
				fileMismatch++
			}
		}
		if err := sc.Err(); err != nil {
			fmt.Fprintf(stderr, "md5sum: %s: %s\n", cf, ioErr(err))
			exit = 1
		}

		if processed == 0 && fileFormatErr > 0 {
			fmt.Fprintf(stderr, "md5sum: %s: no properly formatted checksum lines found\n", cf)
			exit = 1
		}

		totalFormatErr += fileFormatErr
		totalReadErr += fileReadErr
		totalMismatch += fileMismatch
	}

	// Summary wording follows coreutils.
	if !opts.status {
		if totalFormatErr > 0 {
			fmt.Fprintf(stderr, "md5sum: WARNING: %d line%s %s improperly formatted\n",
				totalFormatErr, plural(totalFormatErr), isAre(totalFormatErr))
		}
		if totalReadErr > 0 {
			fmt.Fprintf(stderr, "md5sum: WARNING: %d listed file%s could not be read\n",
				totalReadErr, plural(totalReadErr))
		}
		if totalMismatch > 0 {
			fmt.Fprintf(stderr, "md5sum: WARNING: %d computed checksum%s did NOT match\n",
				totalMismatch, plural(totalMismatch))
		}
	}

	if totalReadErr > 0 || totalMismatch > 0 || totalFormatErr > 0 {
		exit = 1
	}
	return exit
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// ioErr drops the "open <path>:" prefix from os errors.
func ioErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
