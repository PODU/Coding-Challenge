// Package cli parses the mystrace command line.
package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// Config is a parsed command line.
type Config struct {
	Command []string // program and args; empty with -p
	PID     int      // only meaningful when HasPID is set
	HasPID  bool
	Exprs   []string // raw -e values, e.g. "trace=open,read"
	Count   bool     // -c: print a summary instead of each call
	Output  string   // -o file; "" means stderr
	StrSize int      // -s
	Follow  bool     // -f; accepted but not implemented yet
	Help    bool
}

// DefaultStrSize is strace's default for -s.
const DefaultStrSize = 32

// Parse parses os.Args[1:]. For -h it returns a Config with only Help set.
func Parse(args []string) (*Config, error) {
	cfg := &Config{StrSize: DefaultStrSize}

	i := 0
	// optValue returns an option's value, written either as -pVALUE or -p VALUE.
	optValue := func(a string) (string, error) {
		if len(a) > 2 {
			return a[2:], nil
		}
		if i+1 < len(args) {
			i++
			return args[i], nil
		}
		return "", fmt.Errorf("option %s requires a value", a[:2])
	}

	for i < len(args) {
		a := args[i]

		if a == "--" {
			i++
			break
		}
		if a == "-h" || a == "--help" {
			cfg.Help = true
			return cfg, nil
		}

		// The command starts at the first non-option ("-" counts as one).
		if len(a) < 2 || a[0] != '-' {
			break
		}

		switch {
		case a == "-c":
			cfg.Count = true
		case a == "-f":
			cfg.Follow = true
		case strings.HasPrefix(a, "-p"):
			v, err := optValue(a)
			if err != nil {
				return nil, err
			}
			pid, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("invalid PID %q", v)
			}
			cfg.PID = pid
			cfg.HasPID = true
		case strings.HasPrefix(a, "-e"):
			v, err := optValue(a)
			if err != nil {
				return nil, err
			}
			cfg.Exprs = append(cfg.Exprs, v)
		case strings.HasPrefix(a, "-o"):
			v, err := optValue(a)
			if err != nil {
				return nil, err
			}
			cfg.Output = v
		case strings.HasPrefix(a, "-s"):
			v, err := optValue(a)
			if err != nil {
				return nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid -s size %q", v)
			}
			cfg.StrSize = n
		default:
			return nil, fmt.Errorf("unknown option: %s", a)
		}
		i++
	}

	if i < len(args) {
		cfg.Command = append([]string(nil), args[i:]...)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch {
	case c.HasPID && len(c.Command) > 0:
		return fmt.Errorf("cannot combine -p PID with a command")
	case !c.HasPID && len(c.Command) == 0:
		return fmt.Errorf("must specify a command to trace or -p PID")
	}
	return nil
}
