// Package cli parses mygrep's command line.
package cli

import (
	"fmt"
	"strings"
)

type Config struct {
	Pattern     string
	Paths       []string
	Recursive   bool // -r
	IgnoreCase  bool // -i
	Invert      bool // -v
	Count       bool // -c
	LineNumbers bool // -n
	WordMatch   bool // -w
	Help        bool // -h
}

const Usage = `Usage: mygrep [OPTIONS] PATTERN [FILE...]

Search each FILE (or standard input) for lines matching the regular
expression PATTERN.

Options:
  -r, --recursive      Recurse into directories.
  -i, --ignore-case    Case-insensitive matching.
  -v, --invert-match   Select non-matching lines.
  -c, --count          Print only a count of matching lines per file.
  -n, --line-number    Prefix each line of output with its line number.
  -w, --word-regexp    Match only whole words.
  -h, --help           Show this help and exit.

Exit status: 0 if a line was selected, 1 if none, 2 on error.
`

// Parse reads the pattern and paths from args. Short flags can be clustered
// (-ivn), and "--" lets the pattern start with a dash.
func Parse(args []string) (*Config, error) {
	cfg := &Config{}
	var positionals []string
	optsDone := false

	for _, a := range args {
		if !optsDone {
			switch {
			case a == "--":
				optsDone = true
				continue
			case a == "-": // stdin
				positionals = append(positionals, a)
				continue
			case strings.HasPrefix(a, "--"):
				if err := applyLong(cfg, a); err != nil {
					return nil, err
				}
				continue
			case strings.HasPrefix(a, "-") && len(a) > 1:
				if err := applyShortCluster(cfg, a[1:]); err != nil {
					return nil, err
				}
				continue
			}
		}
		positionals = append(positionals, a)
	}

	if cfg.Help {
		return cfg, nil
	}
	if len(positionals) == 0 {
		return nil, fmt.Errorf("no pattern given")
	}
	cfg.Pattern = positionals[0]
	cfg.Paths = positionals[1:]
	return cfg, nil
}

func applyLong(cfg *Config, a string) error {
	switch a {
	case "--recursive":
		cfg.Recursive = true
	case "--ignore-case":
		cfg.IgnoreCase = true
	case "--invert-match":
		cfg.Invert = true
	case "--count":
		cfg.Count = true
	case "--line-number":
		cfg.LineNumbers = true
	case "--word-regexp":
		cfg.WordMatch = true
	case "--help":
		cfg.Help = true
	default:
		return fmt.Errorf("unknown option: %s", a)
	}
	return nil
}

func applyShortCluster(cfg *Config, cluster string) error {
	for _, c := range cluster {
		switch c {
		case 'r':
			cfg.Recursive = true
		case 'i':
			cfg.IgnoreCase = true
		case 'v':
			cfg.Invert = true
		case 'c':
			cfg.Count = true
		case 'n':
			cfg.LineNumbers = true
		case 'w':
			cfg.WordMatch = true
		case 'h':
			cfg.Help = true
		default:
			return fmt.Errorf("unknown option: -%c", c)
		}
	}
	return nil
}
