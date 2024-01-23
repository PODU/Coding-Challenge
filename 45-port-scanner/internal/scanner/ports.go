package scanner

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// MinPort and MaxPort are the inclusive bounds of valid TCP ports.
	MinPort = 1
	MaxPort = 65535
)

// ParsePorts expands a port spec into a sorted list without duplicates:
//
//	"80"             -> [80]
//	"22,80,443"      -> [22, 80, 443]
//	"1-5"            -> [1, 2, 3, 4, 5]
//	"22,80,8000-8002"-> [22, 80, 8000, 8001, 8002]
//	""               -> every port, 1..65535
func ParsePorts(spec string) ([]int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		ports := make([]int, 0, MaxPort)
		for p := MinPort; p <= MaxPort; p++ {
			ports = append(ports, p)
		}
		return ports, nil
	}

	set := make(map[int]struct{})
	for _, raw := range strings.Split(spec, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return nil, fmt.Errorf("empty port token in %q", spec)
		}

		if strings.Contains(tok, "-") {
			low, high, err := parseRange(tok)
			if err != nil {
				return nil, err
			}
			for p := low; p <= high; p++ {
				set[p] = struct{}{}
			}
			continue
		}

		p, err := parsePort(tok)
		if err != nil {
			return nil, err
		}
		set[p] = struct{}{}
	}

	ports := make([]int, 0, len(set))
	for p := range set {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports, nil
}

func parsePort(tok string) (int, error) {
	p, err := strconv.Atoi(tok)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: not a number", tok)
	}
	if p < MinPort || p > MaxPort {
		return 0, fmt.Errorf("port %d out of range (%d-%d)", p, MinPort, MaxPort)
	}
	return p, nil
}

func parseRange(tok string) (int, int, error) {
	parts := strings.Split(tok, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid port range %q: expected LOW-HIGH", tok)
	}
	lowStr := strings.TrimSpace(parts[0])
	highStr := strings.TrimSpace(parts[1])
	if lowStr == "" || highStr == "" {
		return 0, 0, fmt.Errorf("invalid port range %q: empty bound", tok)
	}
	low, err := parsePort(lowStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range %q: %w", tok, err)
	}
	high, err := parsePort(highStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range %q: %w", tok, err)
	}
	if low > high {
		return 0, 0, fmt.Errorf("invalid range %q: low %d > high %d", tok, low, high)
	}
	return low, high, nil
}
