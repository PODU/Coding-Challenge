// Command ccnc is a netcat clone. It connects to or listens on a TCP or UDP
// port and copies stdin to the socket and the socket to stdout. It can also
// scan ports (-z), run a program on the connection (-e) and hex-dump the
// traffic (-x).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const usage = `ccnc [options] [host] [port]

Modes:
  ccnc host port              TCP connect (client)
  ccnc -l -p port             TCP listen  (server)
  ccnc -u host port           UDP client
  ccnc -u -l -p port          UDP server
  ccnc -z host port[-port]    port scan (no data sent)

Options:
  -l            listen for an incoming connection
  -p port       local/remote port number
  -u            UDP instead of TCP
  -z            zero-I/O mode (scan)
  -e proc       execute proc, pipe its stdio over the connection
  -x            hex-dump traffic to stderr
  -v            verbose
  -h            help
`

type Config struct {
	Listen  bool   // -l
	UDP     bool   // -u
	Zero    bool   // -z
	Verbose bool   // -v
	Hex     bool   // -x
	Help    bool   // -h
	Exec    string // -e <proc>
	Host    string // in listen mode, the address to bind
	Port    string // with -z, may be a range "lo-hi"
}

func (c *Config) network() string {
	if c.UDP {
		return "udp"
	}
	return "tcp"
}

// parseArgs parses the arguments after the program name. Flags can be
// grouped, as in -lvp 8888.
func parseArgs(args []string) (*Config, error) {
	cfg := &Config{}
	var positional []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 1 && a[0] == '-' {
			chars := a[1:]
			for j := 0; j < len(chars); j++ {
				c := chars[j]
				switch c {
				case 'l':
					cfg.Listen = true
				case 'u':
					cfg.UDP = true
				case 'z':
					cfg.Zero = true
				case 'v':
					cfg.Verbose = true
				case 'x':
					cfg.Hex = true
				case 'h':
					cfg.Help = true
				case 'p', 'e':
					// The value is the rest of this argument, or the next one.
					val := chars[j+1:]
					if val == "" {
						if i+1 >= len(args) {
							return nil, fmt.Errorf("flag -%c requires a value", c)
						}
						i++
						val = args[i]
					}
					if c == 'p' {
						cfg.Port = val
					} else {
						cfg.Exec = val
					}
					j = len(chars)
				default:
					return nil, fmt.Errorf("unknown flag -%c", c)
				}
			}
		} else {
			positional = append(positional, a)
		}
	}

	if cfg.Help {
		return cfg, nil
	}

	resolveHostPort(cfg, positional)
	return cfg, validate(cfg)
}

// resolveHostPort fills in Host and Port from the positional arguments. When
// listening, the host is optional; -p, if given, takes the place of the
// positional port.
func resolveHostPort(cfg *Config, positional []string) {
	n := len(positional)
	if cfg.Listen {
		if cfg.Port == "" {
			if n >= 1 {
				cfg.Port = positional[n-1]
			}
			if n >= 2 {
				cfg.Host = positional[n-2]
			}
		} else if n >= 1 {
			cfg.Host = positional[0]
		}
		return
	}
	if cfg.Port == "" {
		if n >= 1 {
			cfg.Host = positional[0]
		}
		if n >= 2 {
			cfg.Port = positional[1]
		}
	} else if n >= 1 {
		cfg.Host = positional[0]
	}
}

func validate(cfg *Config) error {
	if cfg.Listen {
		if cfg.Port == "" {
			return fmt.Errorf("listen mode requires a port (-p)")
		}
		if cfg.Zero {
			return fmt.Errorf("-z cannot be combined with -l")
		}
		_, err := parsePort(cfg.Port)
		return err
	}
	if cfg.Host == "" || cfg.Port == "" {
		return fmt.Errorf("client mode requires a host and a port")
	}
	if cfg.Zero {
		_, _, err := parsePortRange(cfg.Port)
		return err
	}
	_, err := parsePort(cfg.Port)
	return err
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q (must be 1..65535)", s)
	}
	return p, nil
}

// parsePortRange parses a port or an inclusive range "lo-hi".
func parsePortRange(s string) (lo, hi int, err error) {
	if i := strings.IndexByte(s, '-'); i >= 0 {
		lo, err = parsePort(s[:i])
		if err != nil {
			return 0, 0, err
		}
		hi, err = parsePort(s[i+1:])
		if err != nil {
			return 0, 0, err
		}
		if lo > hi {
			return 0, 0, fmt.Errorf("invalid range %q (lo > hi)", s)
		}
		return lo, hi, nil
	}
	p, err := parsePort(s)
	if err != nil {
		return 0, 0, err
	}
	return p, p, nil
}

// run is main without the os globals, so tests can pass their own streams.
func run(cfg *Config, stdin io.Reader, stdout, stderr io.Writer) error {
	if cfg.Help {
		fmt.Fprint(stdout, usage)
		return nil
	}
	switch {
	case cfg.Zero:
		return runScan(cfg, stdout, stderr)
	case cfg.Listen:
		return runListen(cfg, stdin, stdout, stderr)
	default:
		return runConnect(cfg, stdin, stdout, stderr)
	}
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ccnc:", err)
		fmt.Fprint(os.Stderr, "\n", usage)
		os.Exit(1)
	}
	if err := run(cfg, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ccnc:", err)
		os.Exit(1)
	}
}
