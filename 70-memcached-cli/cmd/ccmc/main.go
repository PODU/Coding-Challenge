// Command ccmc is a memcached text-protocol CLI. It runs a single command
// (one-shot) or, when no command is supplied, an interactive REPL.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/codingchallenges/ccmc/internal/cli"
	"github.com/codingchallenges/ccmc/internal/client"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run does the work of main and returns the exit code.
func run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts, rest, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(stdout, cli.Usage)
			return cli.ExitOK
		}
		fmt.Fprintf(stderr, "%v\n\n", err)
		fmt.Fprint(stderr, cli.Usage)
		return cli.ExitUsage
	}

	g := cli.Globals{
		Flags:   uint32(opts.flags),
		Exptime: opts.exptime,
		NoReply: opts.noreply,
		Quiet:   opts.quiet,
	}

	dial := func() (*client.Client, error) {
		return client.Dial(opts.host, strconv.Itoa(opts.port), opts.timeout)
	}

	// No command: start the REPL.
	if len(rest) == 0 {
		c, err := dial()
		if err != nil {
			fmt.Fprintf(stderr, "cannot connect to %s:%d: %v\n", opts.host, opts.port, err)
			return cli.ExitProtocol
		}
		defer c.Close()
		if err := cli.REPL(c, g, stdin, stdout, stderr, dial); err != nil {
			fmt.Fprintf(stderr, "repl error: %v\n", err)
			return cli.ExitProtocol
		}
		return cli.ExitOK
	}

	// Parse before dialing so usage errors are reported without a server.
	cmd, err := cli.ParseCommand(rest, g, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return cli.ExitUsage
	}

	c, err := dial()
	if err != nil {
		fmt.Fprintf(stderr, "cannot connect to %s:%d: %v\n", opts.host, opts.port, err)
		return cli.ExitProtocol
	}
	defer c.Close()

	return cli.Execute(c, cmd, stdout, stderr)
}

type options struct {
	host    string
	port    int
	timeout time.Duration
	flags   uint
	exptime int64
	noreply bool
	quiet   bool
}

var errHelp = errors.New("help requested")

// parseFlags parses the global flags, which must come before the command, and
// returns the remaining arguments. It is hand-written because the flag package
// can't take "-?" or "--?" as help.
func parseFlags(args []string) (options, []string, error) {
	opts := options{
		host:    "localhost",
		port:    11211,
		timeout: 5 * time.Second,
	}

	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if len(a) == 0 || a[0] != '-' {
			break
		}
		switch a {
		case "-h", "--help", "-?", "--?", "-help":
			return opts, nil, errHelp
		case "-noreply":
			opts.noreply = true
			continue
		case "-q":
			opts.quiet = true
			continue
		}

		name, val, hasInline := splitFlag(a)
		next := func() (string, error) {
			if hasInline {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s requires a value", name)
			}
			i++
			return args[i], nil
		}

		switch name {
		case "-host":
			v, err := next()
			if err != nil {
				return opts, nil, err
			}
			opts.host = v
		case "-p", "-port":
			v, err := next()
			if err != nil {
				return opts, nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 65535 {
				return opts, nil, fmt.Errorf("invalid port %q", v)
			}
			opts.port = n
		case "-t", "-timeout":
			v, err := next()
			if err != nil {
				return opts, nil, err
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid timeout %q: %v", v, err)
			}
			opts.timeout = d
		case "-flags":
			v, err := next()
			if err != nil {
				return opts, nil, err
			}
			n, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid flags %q", v)
			}
			opts.flags = uint(n)
		case "-exptime":
			v, err := next()
			if err != nil {
				return opts, nil, err
			}
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid exptime %q", v)
			}
			opts.exptime = n
		default:
			return opts, nil, fmt.Errorf("unknown flag %q", a)
		}
	}

	return opts, args[i:], nil
}

// splitFlag splits "-name=value" into its two halves.
func splitFlag(a string) (name, val string, hasInline bool) {
	for i := 1; i < len(a); i++ {
		if a[i] == '=' {
			return a[:i], a[i+1:], true
		}
	}
	return a, "", false
}
