package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config is built from defaults, then the config file, then flags.
type Config struct {
	Prefix byte
	Socket string
}

func defaultConfig() Config {
	return Config{Prefix: 0x02, Socket: defaultSocket()}
}

// ParsePrefix converts a tmux-style key name such as "C-b" or "C-a" into its
// control byte. A bare letter is treated as Ctrl+<letter>.
func ParsePrefix(s string) (byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty prefix")
	}
	up := strings.ToUpper(s)
	var letter byte
	switch {
	case strings.HasPrefix(up, "C-") && len(up) == 3:
		letter = up[2]
	case strings.HasPrefix(up, "^") && len(up) == 2:
		letter = up[1]
	case len(up) == 1:
		letter = up[0]
	default:
		return 0, fmt.Errorf("unrecognised prefix key: %q", s)
	}
	if letter < 'A' || letter > 'Z' {
		return 0, fmt.Errorf("prefix must be a letter: %q", s)
	}
	return letter - 'A' + 1, nil
}

// LoadConfigFile applies `set prefix C-a` lines from path to cfg. Other lines
// are ignored, and a missing file is fine.
func LoadConfigFile(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if (fields[0] == "set" || fields[0] == "set-option") && len(fields) >= 3 && fields[1] == "prefix" {
			p, err := ParsePrefix(fields[2])
			if err != nil {
				return fmt.Errorf("%s:%d: %w", path, line, err)
			}
			cfg.Prefix = p
		}
	}
	return sc.Err()
}

func defaultSocket() string {
	if v := os.Getenv("CCTMUX_SOCK"); v != "" {
		return v
	}
	return os.TempDir() + string(os.PathSeparator) + "cctmux-default.sock"
}
