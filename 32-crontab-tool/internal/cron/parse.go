// Package cron parses five-field cron expressions, describes them and works
// out when they next fire.
package cron

import (
	"fmt"
	"strconv"
	"strings"
)

// Schedule is a parsed cron expression. Each field is a bitset where bit i
// means value i is selected.
type Schedule struct {
	Minute uint64 // bits 0-59
	Hour   uint64 // bits 0-23
	Dom    uint64 // bits 1-31
	Month  uint64 // bits 1-12
	Dow    uint64 // bits 0-6 (Sunday = 0)

	// Whether the day fields were something other than "*". When both are,
	// cron fires if either one matches (see dayMatches).
	DomRestricted bool
	DowRestricted bool

	Raw string // the expression as given, trimmed
}

type fieldSpec struct {
	name  string
	min   int
	max   int
	names map[string]int // JAN, SUN, ... where allowed
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

var (
	minuteSpec = fieldSpec{name: "minute", min: 0, max: 59}
	hourSpec   = fieldSpec{name: "hour", min: 0, max: 23}
	domSpec    = fieldSpec{name: "day-of-month", min: 1, max: 31}
	monthSpec  = fieldSpec{name: "month", min: 1, max: 12, names: monthNames}
	// 7 is accepted as another way to write Sunday.
	dowSpec = fieldSpec{name: "day-of-week", min: 0, max: 7, names: dowNames}
)

// Parse parses a five-field cron expression or an @macro.
func Parse(expr string) (*Schedule, error) {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return nil, fmt.Errorf("empty cron expression")
	}

	fields := strings.Fields(raw)
	if strings.HasPrefix(fields[0], "@") {
		if len(fields) != 1 {
			return nil, fmt.Errorf("macro %q does not take additional fields", fields[0])
		}
		exp, err := expandMacro(fields[0])
		if err != nil {
			return nil, err
		}
		fields = strings.Fields(exp)
	}

	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d: %q", len(fields), raw)
	}

	s := &Schedule{Raw: raw}
	var err error
	if s.Minute, _, err = parseField(fields[0], minuteSpec); err != nil {
		return nil, err
	}
	if s.Hour, _, err = parseField(fields[1], hourSpec); err != nil {
		return nil, err
	}
	if s.Dom, s.DomRestricted, err = parseField(fields[2], domSpec); err != nil {
		return nil, err
	}
	if s.Month, _, err = parseField(fields[3], monthSpec); err != nil {
		return nil, err
	}
	if s.Dow, s.DowRestricted, err = parseField(fields[4], dowSpec); err != nil {
		return nil, err
	}

	// Fold day-of-week 7 into 0.
	if s.Dow&(uint64(1)<<7) != 0 {
		s.Dow = (s.Dow &^ (uint64(1) << 7)) | 1
	}

	return s, nil
}

// parseField parses a comma-separated field into a bitset. restricted is false
// only for a bare "*".
func parseField(field string, spec fieldSpec) (bits uint64, restricted bool, err error) {
	if field == "" {
		return 0, false, fmt.Errorf("%s field is empty", spec.name)
	}
	if i := strings.IndexAny(field, "LW?#"); i >= 0 {
		return 0, false, fmt.Errorf(
			"%s field %q uses unsupported syntax %q (L, W, ?, # are not supported in this version)",
			spec.name, field, string(field[i]))
	}

	restricted = field != "*"
	for _, term := range strings.Split(field, ",") {
		if term == "" {
			return 0, false, fmt.Errorf("%s field %q has an empty list element", spec.name, field)
		}
		tb, err := parseTerm(term, spec)
		if err != nil {
			return 0, false, err
		}
		bits |= tb
	}
	return bits, restricted, nil
}

// parseTerm parses "*", "a" or "a-b", each optionally followed by "/step".
// "a/step" runs from a to the end of the field's range.
func parseTerm(term string, spec fieldSpec) (uint64, error) {
	pieces := strings.Split(term, "/")
	if len(pieces) > 2 {
		return 0, fmt.Errorf("%s field term %q has more than one '/'", spec.name, term)
	}

	step := 1
	hasStep := len(pieces) == 2
	if hasStep {
		if pieces[1] == "" {
			return 0, fmt.Errorf("%s field term %q is missing a step value", spec.name, term)
		}
		v, err := strconv.Atoi(pieces[1])
		if err != nil {
			return 0, fmt.Errorf("%s field term %q has a non-numeric step %q", spec.name, term, pieces[1])
		}
		if v < 1 {
			return 0, fmt.Errorf("%s field term %q step must be >= 1", spec.name, term)
		}
		step = v
	}

	rangeStr := pieces[0]
	var lo, hi int
	switch {
	case rangeStr == "*":
		lo, hi = spec.min, spec.max
	case strings.Contains(rangeStr, "-"):
		bounds := strings.SplitN(rangeStr, "-", 2)
		var err error
		if lo, err = parseValue(bounds[0], spec); err != nil {
			return 0, err
		}
		if hi, err = parseValue(bounds[1], spec); err != nil {
			return 0, err
		}
		if lo > hi {
			return 0, fmt.Errorf("%s field range %q is reversed (%d > %d)", spec.name, rangeStr, lo, hi)
		}
	default:
		v, err := parseValue(rangeStr, spec)
		if err != nil {
			return 0, err
		}
		lo = v
		if hasStep {
			hi = spec.max
		} else {
			hi = v
		}
	}

	var bits uint64
	for i := lo; i <= hi; i += step {
		bits |= uint64(1) << uint(i)
	}
	return bits, nil
}

// parseValue parses a number or name and checks it is in range.
func parseValue(tok string, spec fieldSpec) (int, error) {
	if tok == "" {
		return 0, fmt.Errorf("%s field has an empty value", spec.name)
	}
	if spec.names != nil {
		if v, ok := spec.names[strings.ToUpper(tok)]; ok {
			return v, nil
		}
	}
	n, err := strconv.Atoi(tok)
	if err != nil {
		return 0, fmt.Errorf("%s field has invalid value %q", spec.name, tok)
	}
	if n < spec.min || n > spec.max {
		return 0, fmt.Errorf("%s field value %d out of range [%d-%d]", spec.name, n, spec.min, spec.max)
	}
	return n, nil
}
