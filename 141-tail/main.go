// Command cctail prints the last part of files, and can follow them as they
// grow, get truncated, or get rotated. Coding Challenge #141.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "cctail 1.0.0"

const usage = `Usage: cctail [OPTION]... [FILE]...

Print the last 10 lines of each FILE to standard output. With more than one
FILE, precede each with a header giving the file name. With no FILE, or when
FILE is '-', read standard input.

Options:
  -n, --lines=[+]NUM   output the last NUM lines (default 10);
                       with '+', output starting with line NUM
  -c, --bytes=[+]NUM   output the last NUM bytes;
                       with '+', output starting with byte NUM
  -f, --follow[=HOW]   output appended data as the file grows;
                       HOW is 'descriptor' (default) or 'name'
  -F                   same as --follow=name --retry
      --retry          keep trying to open a file if it is inaccessible
  -q, --quiet, --silent  never output headers giving file names
  -v, --verbose        always output headers giving file names
  -s, --sleep-interval=N  with -f, poll at most every N seconds (default 1.0)
      --pid=PID        with -f, terminate after process PID dies
      --poll           with -f, disable OS file-change notifications
  -h, --help           display this help and exit
      --version        output version information and exit

NUM may have a multiplier suffix: b 512, kB 1000, K 1024, MB 1000*1000,
M 1024*1024, GB 1000^3, G 1024^3. The obsolete form -NUM means -n NUM.

Examples:
  cctail test.log               last 10 lines
  cctail -n 5 test.log          last 5 lines
  cctail -n +5 test.log         from line 5 to the end
  cctail -c 4 bytes.log         last 4 bytes
  seq 1 100 | cctail -n 3       last 3 lines of stdin
  cctail -F app.log             follow app.log across log rotation
  cctail -q -f a.log b.log      follow two files without headers
`

type unit int

const (
	unitLines unit = iota
	unitBytes
)

type headerMode int

const (
	headerAuto headerMode = iota // only when more than one file
	headerNever
	headerAlways
)

type config struct {
	unit       unit
	count      int64
	fromStart  bool // '+NUM': count from the beginning
	follow     bool
	followName bool // -F / --follow=name: reopen the path when it is replaced
	retry      bool
	headers    headerMode
	interval   time.Duration
	pid        int
	poll       bool
	files      []string
}

// usageError makes main exit 1 and point at --help.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func parseArgs(args []string) (*config, error) {
	cfg := &config{unit: unitLines, count: 10, interval: time.Second}
	onlyFiles := false

	setCount := func(u unit, v, what string) error {
		cfg.unit = u
		cfg.fromStart = strings.HasPrefix(v, "+")
		n, err := parseNum(strings.TrimPrefix(strings.TrimPrefix(v, "+"), "-"))
		if err != nil {
			return &usageError{fmt.Sprintf("invalid number of %s: '%s'", what, v)}
		}
		cfg.count = n
		return nil
	}
	setInterval := func(v string) error {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 {
			return &usageError{fmt.Sprintf("invalid number of seconds: '%s'", v)}
		}
		cfg.interval = time.Duration(f * float64(time.Second))
		return nil
	}
	setPid := func(v string) error {
		p, err := strconv.Atoi(v)
		if err != nil || p <= 0 {
			return &usageError{fmt.Sprintf("invalid PID: '%s'", v)}
		}
		cfg.pid = p
		return nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		// value returns the option's argument: inline after '=' or the next arg.
		value := func(name, inline string, hasInline bool) (string, error) {
			if hasInline {
				return inline, nil
			}
			if i+1 >= len(args) {
				return "", &usageError{fmt.Sprintf("option requires an argument -- '%s'", name)}
			}
			i++
			return args[i], nil
		}

		switch {
		case onlyFiles || a == "-" || !strings.HasPrefix(a, "-"):
			cfg.files = append(cfg.files, a)

		case a == "--":
			onlyFiles = true

		case strings.HasPrefix(a, "--"):
			name, inline, hasInline := strings.Cut(a[2:], "=")
			var err error
			switch name {
			case "lines", "bytes":
				var v string
				if v, err = value(name, inline, hasInline); err == nil {
					u := map[string]unit{"lines": unitLines, "bytes": unitBytes}[name]
					err = setCount(u, v, name)
				}
			case "follow":
				cfg.follow = true
				switch {
				case !hasInline || inline == "descriptor":
					cfg.followName = false
				case inline == "name":
					cfg.followName = true
				default:
					err = &usageError{fmt.Sprintf("invalid argument '%s' for '--follow'", inline)}
				}
			case "retry":
				cfg.retry = true
			case "quiet", "silent":
				cfg.headers = headerNever
			case "verbose":
				cfg.headers = headerAlways
			case "sleep-interval":
				var v string
				if v, err = value(name, inline, hasInline); err == nil {
					err = setInterval(v)
				}
			case "pid":
				var v string
				if v, err = value(name, inline, hasInline); err == nil {
					err = setPid(v)
				}
			case "poll":
				cfg.poll = true
			case "help":
				return nil, errHelp
			case "version":
				return nil, errVersion
			default:
				err = &usageError{fmt.Sprintf("unrecognized option '%s'", a)}
			}
			if err != nil {
				return nil, err
			}

		case isAllDigits(a[1:]):
			// Obsolete form: -NUM is -n NUM.
			if err := setCount(unitLines, a[1:], "lines"); err != nil {
				return nil, err
			}

		default:
			// Short options, possibly grouped: -fq, -n5, -fn 5.
			for j := 1; j < len(a); j++ {
				c := a[j]
				rest := a[j+1:]
				var err error
				switch c {
				case 'n', 'c', 's':
					var v string
					if v, err = value(string(c), rest, rest != ""); err != nil {
						return nil, err
					}
					switch c {
					case 'n':
						err = setCount(unitLines, v, "lines")
					case 'c':
						err = setCount(unitBytes, v, "bytes")
					case 's':
						err = setInterval(v)
					}
					j = len(a) // the value consumed the rest of this arg
				case 'f':
					cfg.follow = true
				case 'F':
					cfg.follow, cfg.followName, cfg.retry = true, true, true
				case 'q':
					cfg.headers = headerNever
				case 'v':
					cfg.headers = headerAlways
				case 'h':
					return nil, errHelp
				default:
					err = &usageError{fmt.Sprintf("invalid option -- '%c'", c)}
				}
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg, nil
}

var (
	errHelp    = fmt.Errorf("help")
	errVersion = fmt.Errorf("version")
)

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseNum parses a non-negative count with an optional size suffix.
func parseNum(s string) (int64, error) {
	suffixes := []struct {
		s string
		m int64
	}{
		{"kB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"b", 512}, {"K", 1024}, {"k", 1024}, {"M", 1 << 20}, {"G", 1 << 30},
	}
	mult := int64(1)
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf.s) && len(s) > len(suf.s) {
			s, mult = strings.TrimSuffix(s, suf.s), suf.m
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("bad number")
	}
	if mult > 1 && n > (1<<62)/mult {
		return 1 << 62, nil // saturate: "more than the file holds"
	}
	return n * mult, nil
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	switch {
	case err == errHelp:
		fmt.Print(usage)
		return
	case err == errVersion:
		fmt.Println(version)
		return
	case err != nil:
		fmt.Fprintf(os.Stderr, "cctail: %v\nTry 'cctail --help' for more information.\n", err)
		os.Exit(1)
	}

	// Ctrl+C / SIGTERM cancel the context; the follow loop then closes every
	// file handle and watcher and returns normally so output is flushed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := run(ctx, cfg, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, cfg *config, stdin *os.File, stdout, stderr io.Writer) int {
	t := &tailer{
		cfg:    cfg,
		stdin:  stdin,
		out:    newPrinter(stdout, cfg.headers == headerAlways || (cfg.headers == headerAuto && len(cfg.files) > 1)),
		stderr: stderr,
	}
	return t.run(ctx)
}
