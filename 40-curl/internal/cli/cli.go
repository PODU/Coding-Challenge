// Package cli parses cc-curl's arguments. It handles curl's flag styles: -H x,
// -Hx, grouped flags like -vL, --header x and --header=x.
package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Options holds the parsed command line.
type Options struct {
	URL        string
	Method     string   // -X; "" if not given
	Headers    []string // raw -H values, in order
	Data       string
	HasData    bool
	Verbose    bool
	Include    bool   // -i
	Output     string // -o; "" means stdout
	Location   bool   // -L
	Insecure   bool   // -k
	MaxRedirs  int
	HelpWanted bool
}

// ErrHelp is returned for -h or --help.
var ErrHelp = errors.New("help requested")

const Usage = `cc-curl - a tiny HTTP/1.1 client that speaks over a raw TCP socket

Usage:
  cc-curl [options] <url>

Options:
  -X, --request <method>   HTTP method (GET default; POST if -d and no -X)
  -H, --header <header>    Add/override a header "Name: value" (repeatable)
  -d, --data <data>        Request body; implies POST unless -X given
  -v, --verbose            Print request/response metadata to stderr (> < *)
  -i, --include            Include response headers in the stdout output
  -o, --output <file>      Write body to <file> instead of stdout
  -L, --location           Follow 3xx redirects (max 20)
  -k, --insecure           Skip TLS certificate verification (https)
      --max-redirs <n>     Override redirect limit (default 20)
  -h, --help               Show this help
`

// Parse parses the arguments after the program name.
func Parse(argv []string) (Options, error) {
	opts := Options{MaxRedirs: 20}
	i := 0
	for i < len(argv) {
		arg := argv[i]
		i++

		switch {
		case arg == "--":
			for ; i < len(argv); i++ {
				if err := setURL(&opts, argv[i]); err != nil {
					return opts, err
				}
			}
			return finalize(opts)

		case len(arg) >= 2 && arg[:2] == "--":
			var err error
			i, err = parseLong(&opts, arg[2:], argv, i)
			if err != nil {
				return opts, err
			}

		case len(arg) >= 1 && arg[0] == '-' && arg != "-":
			var err error
			i, err = parseShortCluster(&opts, arg[1:], argv, i)
			if err != nil {
				return opts, err
			}

		default:
			if err := setURL(&opts, arg); err != nil {
				return opts, err
			}
		}
	}
	return finalize(opts)
}

func finalize(opts Options) (Options, error) {
	if opts.HelpWanted {
		return opts, ErrHelp
	}
	if opts.URL == "" {
		return opts, errors.New("no URL specified")
	}
	return opts, nil
}

func setURL(opts *Options, v string) error {
	if opts.URL != "" {
		return fmt.Errorf("multiple URLs given (%q and %q); only one is supported", opts.URL, v)
	}
	opts.URL = v
	return nil
}

// parseLong handles "--name" or "--name=value" and returns the index of the
// next unread argument.
func parseLong(opts *Options, body string, argv []string, i int) (int, error) {
	name, val, hasInlineVal := strings.Cut(body, "=")

	takeValue := func() (string, error) {
		if hasInlineVal {
			return val, nil
		}
		if i >= len(argv) {
			return "", fmt.Errorf("option --%s requires a value", name)
		}
		v := argv[i]
		i++
		return v, nil
	}

	switch name {
	case "request":
		v, err := takeValue()
		if err != nil {
			return i, err
		}
		opts.Method = v
	case "header":
		v, err := takeValue()
		if err != nil {
			return i, err
		}
		opts.Headers = append(opts.Headers, v)
	case "data":
		v, err := takeValue()
		if err != nil {
			return i, err
		}
		opts.Data = v
		opts.HasData = true
	case "output":
		v, err := takeValue()
		if err != nil {
			return i, err
		}
		opts.Output = v
	case "max-redirs":
		v, err := takeValue()
		if err != nil {
			return i, err
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return i, fmt.Errorf("invalid --max-redirs value %q", v)
		}
		opts.MaxRedirs = n
	case "verbose":
		opts.Verbose = true
	case "include":
		opts.Include = true
	case "location":
		opts.Location = true
	case "insecure":
		opts.Insecure = true
	case "help":
		opts.HelpWanted = true
	default:
		return i, fmt.Errorf("unknown option --%s", name)
	}
	return i, nil
}

// parseShortCluster handles a group like "-vL". A flag that takes a value uses
// the rest of the group (-Hx) or, if there is none, the next argument (-H x).
func parseShortCluster(opts *Options, cluster string, argv []string, i int) (int, error) {
	for j := 0; j < len(cluster); j++ {
		c := cluster[j]
		rest := cluster[j+1:]

		takeValue := func() (string, error) {
			if rest != "" {
				return rest, nil
			}
			if i >= len(argv) {
				return "", fmt.Errorf("option -%c requires a value", c)
			}
			v := argv[i]
			i++
			return v, nil
		}

		switch c {
		case 'X':
			v, err := takeValue()
			if err != nil {
				return i, err
			}
			opts.Method = v
			return i, nil
		case 'H':
			v, err := takeValue()
			if err != nil {
				return i, err
			}
			opts.Headers = append(opts.Headers, v)
			return i, nil
		case 'd':
			v, err := takeValue()
			if err != nil {
				return i, err
			}
			opts.Data = v
			opts.HasData = true
			return i, nil
		case 'o':
			v, err := takeValue()
			if err != nil {
				return i, err
			}
			opts.Output = v
			return i, nil
		case 'v':
			opts.Verbose = true
		case 'i':
			opts.Include = true
		case 'L':
			opts.Location = true
		case 'k':
			opts.Insecure = true
		case 'h':
			opts.HelpWanted = true
		default:
			return i, fmt.Errorf("unknown option -%c", c)
		}
	}
	return i, nil
}
