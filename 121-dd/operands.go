package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// StatusLevel is the status= operand.
type StatusLevel int

const (
	StatusDefault  StatusLevel = iota // full summary at the end
	StatusNone                        // errors only
	StatusNoxfer                      // record counts, no transfer line
	StatusProgress                    // periodic transfer line while copying
)

// ConvFlags holds the conv= conversions.
type ConvFlags struct {
	Ucase   bool
	Lcase   bool
	Swab    bool
	Notrunc bool
	Noerror bool
	Sync    bool
	Sparse  bool
}

// Config is the parsed set of operands.
type Config struct {
	InFile  string
	OutFile string

	Ibs    int64
	Obs    int64
	BsMode bool // bs= was given: each input block is written as one record

	Count int64 // -1 means no limit
	Skip  int64
	Seek  int64

	Conv   ConvFlags
	Status StatusLevel

	Fullblock bool // iflag=fullblock
}

func defaultConfig() *Config {
	return &Config{
		Ibs:   512,
		Obs:   512,
		Count: -1,
	}
}

// ParseOperands parses operand=value arguments into a Config.
func ParseOperands(args []string) (*Config, error) {
	cfg := defaultConfig()

	var bsSet bool
	seen := make(map[string]bool)

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("unrecognized operand %q (dd takes operand=value, not flags)", arg)
		}
		eq := strings.IndexByte(arg, '=')
		if eq < 0 {
			return nil, fmt.Errorf("unrecognized operand %q (expected operand=value)", arg)
		}
		key := arg[:eq]
		val := arg[eq+1:]

		if seen[key] {
			return nil, fmt.Errorf("operand %q specified more than once", key)
		}
		seen[key] = true

		switch key {
		case "if":
			cfg.InFile = val
		case "of":
			cfg.OutFile = val
		case "bs", "ibs", "obs":
			n, err := parseSize(val)
			if err != nil {
				return nil, fmt.Errorf("invalid %s=%s: %w", key, val, err)
			}
			if n <= 0 {
				return nil, fmt.Errorf("invalid %s=%s: block size must be positive", key, val)
			}
			switch {
			case key == "bs":
				cfg.Ibs, cfg.Obs = n, n
				cfg.BsMode = true
				bsSet = true
			case bsSet:
				// bs= takes precedence over ibs= and obs=.
			case key == "ibs":
				cfg.Ibs = n
			default:
				cfg.Obs = n
			}
		case "count", "skip", "seek":
			n, err := parseSize(val)
			if err != nil {
				return nil, fmt.Errorf("invalid %s=%s: %w", key, val, err)
			}
			switch key {
			case "count":
				cfg.Count = n
			case "skip":
				cfg.Skip = n
			default:
				cfg.Seek = n
			}
		case "conv":
			if err := parseConv(val, &cfg.Conv); err != nil {
				return nil, err
			}
		case "status":
			s, err := parseStatus(val)
			if err != nil {
				return nil, err
			}
			cfg.Status = s
		case "iflag":
			if err := parseFlags(val, cfg, true); err != nil {
				return nil, err
			}
		case "oflag":
			if err := parseFlags(val, cfg, false); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown operand %q", key)
		}
	}

	if cfg.Conv.Ucase && cfg.Conv.Lcase {
		return nil, fmt.Errorf("conv=ucase and conv=lcase are mutually exclusive")
	}
	return cfg, nil
}

func parseConv(list string, out *ConvFlags) error {
	for _, tok := range strings.Split(list, ",") {
		switch tok {
		case "ucase":
			out.Ucase = true
		case "lcase":
			out.Lcase = true
		case "swab":
			out.Swab = true
		case "notrunc":
			out.Notrunc = true
		case "noerror":
			out.Noerror = true
		case "sync":
			out.Sync = true
		case "sparse":
			out.Sparse = true
		case "": // stray comma
		default:
			return fmt.Errorf("unknown conversion %q", tok)
		}
	}
	return nil
}

func parseStatus(val string) (StatusLevel, error) {
	switch val {
	case "none":
		return StatusNone, nil
	case "noxfer":
		return StatusNoxfer, nil
	case "progress":
		return StatusProgress, nil
	default:
		return StatusDefault, fmt.Errorf("unknown status %q (want none|noxfer|progress)", val)
	}
}

func parseFlags(list string, cfg *Config, input bool) error {
	for _, tok := range strings.Split(list, ",") {
		switch tok {
		case "fullblock":
			if !input {
				return fmt.Errorf("oflag=fullblock is not valid (use iflag=fullblock)")
			}
			cfg.Fullblock = true
		case "direct", "dsync", "sync", "nonblock", "noatime":
			// Accepted for compatibility but ignored.
		case "":
		default:
			kind := "iflag"
			if !input {
				kind = "oflag"
			}
			return fmt.Errorf("unknown %s %q", kind, tok)
		}
	}
	return nil
}

// parseSize parses a dd size such as "512", "4k" or "2x512". Negative values
// and overflow are rejected.
func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	var product int64 = 1
	for _, p := range strings.Split(s, "x") {
		v, err := parseSizePart(p)
		if err != nil {
			return 0, err
		}
		if v != 0 && product > math.MaxInt64/v {
			return 0, fmt.Errorf("value too large")
		}
		product *= v
	}
	return product, nil
}

var sizeSuffixes = map[byte]int64{
	'c': 1,
	'w': 2,
	'b': 512,
	'k': 1024,
	'K': 1024,
	'M': 1 << 20,
	'G': 1 << 30,
}

func parseSizePart(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	mult := int64(1)
	if m, ok := sizeSuffixes[s[len(s)-1]]; ok {
		mult = m
		s = s[:len(s)-1]
	}
	if s == "" {
		return 0, fmt.Errorf("missing number before suffix")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("not a valid number")
	}
	if n < 0 {
		return 0, fmt.Errorf("negative values are not allowed")
	}
	if n > math.MaxInt64/mult {
		return 0, fmt.Errorf("value too large")
	}
	return n * mult, nil
}
