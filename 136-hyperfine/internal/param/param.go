// Package param expands parameterised command templates into the concrete
// command lines that will actually be benchmarked.
package param

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Scan is a numeric sweep: `-P threads 1 8 -D 2` walks 1, 3, 5, 7.
type Scan struct {
	Var  string
	Min  string
	Max  string
	Step string // empty means 1
}

// List is an explicit set of values: `-L compiler gcc,clang`.
type List struct {
	Var    string
	Values []string
}

// Binding is one point in the parameter space, in declaration order.
type Binding struct {
	Names  []string
	Values map[string]string
}

// Substitute replaces every `{name}` placeholder in s with its bound value.
// Placeholders with no binding are left alone so that a literal brace in a
// command survives untouched.
func (b Binding) Substitute(s string) string {
	if len(b.Values) == 0 {
		return s
	}
	for _, n := range b.Names {
		s = strings.ReplaceAll(s, "{"+n+"}", b.Values[n])
	}
	return s
}

// Expand produces the cartesian product of the given scans and lists. With no
// parameters at all it returns a single empty binding, so callers can always
// iterate over the result without special-casing.
func Expand(scans []Scan, lists []List) ([]Binding, error) {
	if len(scans) > 0 && len(lists) > 0 {
		return nil, fmt.Errorf("--parameter-scan and --parameter-list cannot be used together")
	}
	if len(scans) > 1 {
		return nil, fmt.Errorf("--parameter-scan can only be given once")
	}

	type axis struct {
		name   string
		values []string
	}
	var axes []axis

	for _, s := range scans {
		vals, err := scanValues(s)
		if err != nil {
			return nil, err
		}
		axes = append(axes, axis{name: s.Var, values: vals})
	}
	for _, l := range lists {
		if len(l.Values) == 0 {
			return nil, fmt.Errorf("--parameter-list %s: no values given", l.Var)
		}
		axes = append(axes, axis{name: l.Var, values: l.Values})
	}

	names := make([]string, len(axes))
	for i, a := range axes {
		names[i] = a.name
	}

	out := []Binding{{Names: names, Values: map[string]string{}}}
	for _, a := range axes {
		next := make([]Binding, 0, len(out)*len(a.values))
		for _, b := range out {
			for _, v := range a.values {
				vals := make(map[string]string, len(b.Values)+1)
				for k, old := range b.Values {
					vals[k] = old
				}
				vals[a.name] = v
				next = append(next, Binding{Names: names, Values: vals})
			}
		}
		out = next
	}
	return out, nil
}

// scanValues enumerates a numeric range without accumulating floating-point
// drift: the bounds and step are scaled to integers by the largest number of
// decimal places any of them uses, stepped there, and formatted back.
func scanValues(s Scan) ([]string, error) {
	step := s.Step
	if step == "" {
		step = "1"
	}
	digits := maxDecimals(s.Min, s.Max, step)
	scale := math.Pow10(digits)

	min, err := parseScaled(s.Min, scale)
	if err != nil {
		return nil, fmt.Errorf("--parameter-scan %s: %w", s.Var, err)
	}
	max, err := parseScaled(s.Max, scale)
	if err != nil {
		return nil, fmt.Errorf("--parameter-scan %s: %w", s.Var, err)
	}
	delta, err := parseScaled(step, scale)
	if err != nil {
		return nil, fmt.Errorf("--parameter-step-size: %w", err)
	}
	if delta == 0 {
		return nil, fmt.Errorf("--parameter-step-size: step must not be zero")
	}
	if (max-min)/delta < 0 {
		return nil, fmt.Errorf("--parameter-scan %s: step %s never reaches %s from %s", s.Var, step, s.Max, s.Min)
	}

	const limit = 100000
	var vals []string
	for v := min; (delta > 0 && v <= max) || (delta < 0 && v >= max); v += delta {
		vals = append(vals, formatScaled(v, digits))
		if len(vals) > limit {
			return nil, fmt.Errorf("--parameter-scan %s: range expands to more than %d values", s.Var, limit)
		}
	}
	return vals, nil
}

func maxDecimals(xs ...string) int {
	m := 0
	for _, x := range xs {
		if i := strings.IndexByte(x, '.'); i >= 0 {
			if d := len(x) - i - 1; d > m {
				m = d
			}
		}
	}
	return m
}

func parseScaled(s string, scale float64) (int64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	return int64(math.Round(f * scale)), nil
}

func formatScaled(v int64, digits int) string {
	if digits == 0 {
		return strconv.FormatInt(v, 10)
	}
	return strconv.FormatFloat(float64(v)/math.Pow10(digits), 'f', digits, 64)
}
