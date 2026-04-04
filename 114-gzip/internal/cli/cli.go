// Package cli is gzipcc's command line, modelled on gzip's.
package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gzipcc/internal/gzip"
)

const version = "gzipcc 1.0.0"

const usage = `Usage: gzipcc [OPTION]... [FILE]...
Compress or decompress FILEs (by default, compress FILE in-place to FILE.gz).

  -d, --decompress  decompress
  -k, --keep        keep (don't delete) input files
  -c, --stdout      write to standard output, keep original files
  -f, --force       force overwrite of output and (de)compress to a tty
  -t, --test        test compressed file integrity
  -l, --list        list compressed file contents
  -1 .. -9          compression level (1=fast .. 9=best, default 6)
      --fast        alias for -1
      --best        alias for -9
  -v, --verbose     verbose mode
  -h, --help        give this help
  -V, --version     display version number

With no FILE, or when FILE is -, read standard input.
`

type options struct {
	decompress bool
	keep       bool
	stdout     bool
	force      bool
	test       bool
	list       bool
	verbose    bool
	level      int
	files      []string
}

// Run parses args (without the program name), does the work and returns the
// exit status.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opt := options{level: 6}

	parseErr := func(format string, a ...interface{}) int {
		fmt.Fprintf(stderr, "gzipcc: "+format+"\n", a...)
		return 2
	}

	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			opt.files = append(opt.files, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--decompress", "--uncompress":
				opt.decompress = true
			case "--keep":
				opt.keep = true
			case "--stdout", "--to-stdout":
				opt.stdout = true
			case "--force":
				opt.force = true
			case "--test":
				opt.test = true
			case "--list":
				opt.list = true
			case "--verbose":
				opt.verbose = true
			case "--fast":
				opt.level = 1
			case "--best":
				opt.level = 9
			case "--help":
				fmt.Fprint(stdout, usage)
				return 0
			case "--version":
				fmt.Fprintln(stdout, version)
				return 0
			default:
				return parseErr("unknown option %q", arg)
			}
			continue
		}
		// Short flag cluster, e.g. -dkc or -9.
		for j := 1; j < len(arg); j++ {
			switch ch := arg[j]; ch {
			case 'd':
				opt.decompress = true
			case 'k':
				opt.keep = true
			case 'c':
				opt.stdout = true
			case 'f':
				opt.force = true
			case 't':
				opt.test = true
			case 'l':
				opt.list = true
			case 'v':
				opt.verbose = true
			case 'h':
				fmt.Fprint(stdout, usage)
				return 0
			case 'V':
				fmt.Fprintln(stdout, version)
				return 0
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				opt.level = int(ch - '0')
			default:
				return parseErr("unknown option -%c", ch)
			}
		}
	}
	opt.files = append(opt.files, args[i:]...)

	if opt.stdout {
		opt.keep = true
	}

	a := &app{opt: opt, stdin: stdin, stdout: stdout, stderr: stderr}
	return a.run()
}

type app struct {
	opt    options
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (a *app) warn(format string, args ...interface{}) {
	fmt.Fprintf(a.stderr, "gzipcc: "+format+"\n", args...)
}

func (a *app) run() int {
	if len(a.opt.files) == 0 {
		if err := a.processStdin(); err != nil {
			a.warn("%v", err)
			return 1
		}
		return 0
	}

	exit := 0
	for _, f := range a.opt.files {
		if f == "-" {
			if err := a.processStdin(); err != nil {
				a.warn("%v", err)
				exit = 1
			}
			continue
		}
		if err := a.processFile(f); err != nil {
			a.warn("%v", err)
			exit = 1
		}
	}
	return exit
}

func (a *app) processStdin() error {
	data, err := io.ReadAll(a.stdin)
	if err != nil {
		return err
	}
	switch {
	case a.opt.list:
		return a.listData(data, "<stdin>")
	case a.opt.test:
		var sink bytes.Buffer
		_, err := gzip.Decompress(&sink, bytes.NewReader(data))
		return err
	case a.opt.decompress:
		_, err := gzip.Decompress(a.stdout, bytes.NewReader(data))
		return err
	default:
		return gzip.Compress(a.stdout, data, a.opt.level, gzip.Header{})
	}
}

func (a *app) processFile(name string) error {
	switch {
	case a.opt.list:
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return a.listData(data, name)
	case a.opt.test:
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		var sink bytes.Buffer
		if _, err := gzip.Decompress(&sink, bytes.NewReader(data)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if a.opt.verbose {
			fmt.Fprintf(a.stderr, "%s: OK\n", name)
		}
		return nil
	case a.opt.decompress:
		return a.decompressFile(name)
	default:
		return a.compressFile(name)
	}
}

func (a *app) compressFile(name string) error {
	if strings.HasSuffix(name, ".gz") && !a.opt.force {
		return fmt.Errorf("%s already has .gz suffix -- unchanged", name)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	info, _ := os.Stat(name)
	var mtime uint32
	if info != nil {
		mtime = uint32(info.ModTime().Unix())
	}
	hdr := gzip.Header{Name: filepath.Base(name), ModTime: mtime}

	if a.opt.stdout {
		return gzip.Compress(a.stdout, data, a.opt.level, hdr)
	}

	out := name + ".gz"
	if err := a.checkOutput(out); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gzip.Compress(&buf, data, a.opt.level, hdr); err != nil {
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0644); err != nil {
		return err
	}
	if a.opt.verbose {
		ratio := 0.0
		if len(data) > 0 {
			ratio = 100.0 * (1 - float64(buf.Len())/float64(len(data)))
		}
		fmt.Fprintf(a.stderr, "%s:\t%.1f%% -- replaced with %s\n", name, ratio, out)
	}
	if !a.opt.keep {
		return os.Remove(name)
	}
	return nil
}

func (a *app) decompressFile(name string) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if a.opt.stdout {
		_, err := gzip.Decompress(a.stdout, bytes.NewReader(data))
		return err
	}

	out := outputName(name)
	if out == name {
		return fmt.Errorf("%s: cannot determine output name", name)
	}
	if err := a.checkOutput(out); err != nil {
		return err
	}
	var buf bytes.Buffer
	hdr, err := gzip.Decompress(&buf, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	// Without a .gz suffix to strip, prefer the name stored in the header.
	if hdr.Name != "" && !strings.HasSuffix(name, ".gz") {
		out = filepath.Join(filepath.Dir(name), filepath.Base(hdr.Name))
		if err := a.checkOutput(out); err != nil {
			return err
		}
	}
	if err := os.WriteFile(out, buf.Bytes(), 0644); err != nil {
		return err
	}
	if a.opt.verbose {
		fmt.Fprintf(a.stderr, "%s:\t-- replaced with %s\n", name, out)
	}
	if !a.opt.keep {
		return os.Remove(name)
	}
	return nil
}

func (a *app) checkOutput(out string) error {
	if a.opt.force {
		return nil
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists; use -f to overwrite", out)
	}
	return nil
}

func outputName(name string) string {
	if strings.HasSuffix(name, ".gz") {
		return strings.TrimSuffix(name, ".gz")
	}
	if strings.HasSuffix(name, ".tgz") {
		return strings.TrimSuffix(name, ".tgz") + ".tar"
	}
	return name
}

func (a *app) listData(data []byte, name string) error {
	var sink bytes.Buffer
	if _, err := gzip.Decompress(&sink, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	compressed := len(data)
	uncompressed := sink.Len()
	ratio := 0.0
	if uncompressed > 0 {
		ratio = 100.0 * (1 - float64(compressed)/float64(uncompressed))
	}
	fmt.Fprintf(a.stdout, "compressed  uncompressed  ratio  uncompressed_name\n")
	fmt.Fprintf(a.stdout, "%10d  %12d  %4.1f%%  %s\n", compressed, uncompressed, ratio, outputName(name))
	return nil
}
