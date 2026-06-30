// Command du is a clone of the Unix du (disk usage) tool. It prints the total
// size of each directory, deepest first.
//
// Sizes on Windows are approximate; see README.md.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"du/internal/report"
	"du/internal/walk"
)

var errHelp = errors.New("help requested")

func main() {
	opts, paths, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(os.Stdout, usage)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		os.Exit(2)
	}
	os.Exit(Run(opts, paths, os.Stdout, os.Stderr))
}

// Run reports on every path and returns the exit status: 1 if anything could
// not be read, otherwise 0.
func Run(opts walk.Options, paths []string, out, errOut io.Writer) int {
	w := walk.New(opts, errOut)
	rep := report.New(out, opts)

	var grand int64
	for _, p := range paths {
		entries, total := w.Walk(p)
		rep.Report(entries)
		grand += total
	}
	if opts.Total {
		rep.Total(grand)
	}
	if w.HadError() {
		return 1
	}
	return 0
}

// parseArgs handles GNU-style options: --opt, --opt=val, --opt val, bundled
// short flags (-sh), short flags with values (-d2, -d 2) and "--".
func parseArgs(args []string) (walk.Options, []string, error) {
	o := walk.Options{MaxDepth: -1, Unit: walk.UnitBlock, BlockSize: 512}
	var paths []string
	var excludeFroms []string
	positionalOnly := false

	for i := 0; i < len(args); i++ {
		a := args[i]

		if positionalOnly || a == "-" || !strings.HasPrefix(a, "-") {
			paths = append(paths, a)
			continue
		}
		if a == "--" {
			positionalOnly = true
			continue
		}

		// next returns an option's value: the "=value" part if there was
		// one, otherwise the following argument.
		next := func(inline string, has bool, label string) (string, error) {
			if has {
				return inline, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("option %s requires an argument", label)
			}
			i++
			return args[i], nil
		}

		if strings.HasPrefix(a, "--") {
			name := a[2:]
			val, has := "", false
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				val, has = name[idx+1:], true
				name = name[:idx]
			}
			switch name {
			case "human-readable":
				o.Unit = walk.UnitHuman
			case "summarize":
				o.Summarize = true
			case "all":
				o.All = true
			case "total":
				o.Total = true
			case "dereference":
				o.Dereference = true
			case "one-file-system":
				o.OneFileSystem = true
			case "apparent-size":
				o.Apparent = true
			case "bytes":
				o.Apparent = true
				o.Unit = walk.UnitCustom
				o.BlockSize = 1
			case "inodes":
				o.Inodes = true
			case "json":
				o.JSON = true
			case "max-depth":
				v, err := next(val, has, "--max-depth")
				if err != nil {
					return o, nil, err
				}
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return o, nil, fmt.Errorf("invalid max-depth %q", v)
				}
				o.MaxDepth = n
			case "threshold":
				v, err := next(val, has, "--threshold")
				if err != nil {
					return o, nil, err
				}
				n, err := parseSize(v)
				if err != nil {
					return o, nil, err
				}
				o.Threshold = n
			case "exclude":
				v, err := next(val, has, "--exclude")
				if err != nil {
					return o, nil, err
				}
				o.Excludes = append(o.Excludes, v)
			case "exclude-from":
				v, err := next(val, has, "--exclude-from")
				if err != nil {
					return o, nil, err
				}
				excludeFroms = append(excludeFroms, v)
			case "block-size":
				v, err := next(val, has, "--block-size")
				if err != nil {
					return o, nil, err
				}
				n, err := parseSize(v)
				if err != nil || n <= 0 {
					return o, nil, fmt.Errorf("invalid block-size %q", v)
				}
				o.Unit = walk.UnitCustom
				o.BlockSize = n
			case "sort":
				v, err := next(val, has, "--sort")
				if err != nil {
					return o, nil, err
				}
				o.Sort = v
			case "help":
				return o, nil, errHelp
			default:
				return o, nil, fmt.Errorf("unknown option --%s", name)
			}
			continue
		}

		chars := a[1:]
		for j := 0; j < len(chars); j++ {
			c := chars[j]
			switch c {
			case 'h':
				o.Unit = walk.UnitHuman
			case 'k':
				o.Unit = walk.UnitK
			case 'm':
				o.Unit = walk.UnitM
			case 'a':
				o.All = true
			case 's':
				o.Summarize = true
			case 'c':
				o.Total = true
			case 'L':
				o.Dereference = true
			case 'x':
				o.OneFileSystem = true
			case 'b':
				o.Apparent = true
				o.Unit = walk.UnitCustom
				o.BlockSize = 1
			case 'd', 't':
				v := chars[j+1:]
				if v == "" {
					if i+1 >= len(args) {
						return o, nil, fmt.Errorf("-%c requires an argument", c)
					}
					i++
					v = args[i]
				}
				if c == 'd' {
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 {
						return o, nil, fmt.Errorf("invalid max-depth %q", v)
					}
					o.MaxDepth = n
				} else {
					n, err := parseSize(v)
					if err != nil {
						return o, nil, err
					}
					o.Threshold = n
				}
				j = len(chars)
			default:
				return o, nil, fmt.Errorf("unknown option -%c", c)
			}
		}
	}

	for _, f := range excludeFroms {
		lines, err := readPatternFile(f)
		if err != nil {
			return o, nil, err
		}
		o.Excludes = append(o.Excludes, lines...)
	}

	if o.All && o.Summarize {
		return o, nil, errors.New("cannot both summarize and show all entries")
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	return o, paths, nil
}

// parseSize parses a size with an optional sign and a single 1024-based suffix
// (K/M/G/T/P, case-insensitive). A bare number is bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size")
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	if s == "" {
		return 0, errors.New("empty size")
	}

	mult := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		mult = 1 << 10
	case 'M', 'm':
		mult = 1 << 20
	case 'G', 'g':
		mult = 1 << 30
	case 'T', 't':
		mult = 1 << 40
	case 'P', 'p':
		mult = 1 << 50
	}
	if mult != 1 {
		s = s[:len(s)-1]
	}
	if s == "" {
		return 0, errors.New("missing size value")
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	v := n * mult
	if neg {
		v = -v
	}
	return v, nil
}

// readPatternFile reads --exclude-from globs, one per line, skipping blanks and
// '#' comments.
func readPatternFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

const usage = `Usage: du [OPTIONS] [PATH...]

Recursively sum file sizes per directory (bottom-up) and print "size<TAB>path",
deepest-first. Default unit is 512-byte blocks (POSIX).

Units (last wins):
  -h, --human-readable   sizes with K/M/G/T suffixes (1024-based)
  -k                     1 KiB (1024-byte) blocks
  -m                     1 MiB blocks
      --block-size=SIZE  custom block size (e.g. 1K, 1M)
      --apparent-size    logical file size instead of allocated blocks
  -b, --bytes            apparent size in bytes (implies --apparent-size)

Output selection:
  -a, --all              include individual files (not just directories)
  -s, --summarize        only a total per argument (implies depth 0)
  -d, --max-depth=N      print entries up to N levels deep
  -c, --total            append a grand 'total' line
  -t, --threshold=SIZE   only entries >= SIZE (or <= |SIZE| if negative)

Traversal:
  -L, --dereference      follow symlinks, count target contents
  -x, --one-file-system  stay on one filesystem
      --exclude=PATTERN  skip entries matching glob (repeatable)
      --exclude-from=F   read exclude globs from file F (repeatable)

Misc:
      --inodes           count inodes instead of bytes
      --help             show this help

Platform note: on Unix, allocated sizes come from st_blocks (true on-disk
usage) and hardlinks are de-duplicated by (device, inode). On Windows there is
no block/inode metadata in the standard library, so sizes are the logical size
rounded up to a 4096-byte cluster and hardlink dedup is disabled.
`
