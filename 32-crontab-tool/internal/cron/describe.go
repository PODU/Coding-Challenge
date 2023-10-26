package cron

import (
	"fmt"
	"strconv"
	"strings"
)

var weekdayNames = []string{
	"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
}

var monthFullNames = []string{
	"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// Describe returns an English description of s, e.g.
// "0 9 * * SAT" -> "At 09:00, only on Saturday".
func Describe(s *Schedule) string {
	timePart, freq := describeTime(s)
	return timePart + describeDayClause(s, freq) + describeMonthClause(s)
}

// describeTime describes the minute and hour fields. freq is true for the
// "Every ..." phrasings, which shouldn't be followed by " every day".
func describeTime(s *Schedule) (text string, freq bool) {
	minVals := bitsToList(s.Minute, 0, 59)
	hourVals := bitsToList(s.Hour, 0, 23)
	minAll := len(minVals) == 60
	hourAll := len(hourVals) == 24

	if minAll && hourAll {
		return "Every minute", true
	}
	if step, ok := arithStep(minVals, 0, 59); ok && step > 1 && hourAll {
		return fmt.Sprintf("Every %d minutes", step), true
	}

	if len(minVals) == 1 {
		m := minVals[0]
		if hourAll {
			if m == 0 {
				return "Every hour", true
			}
			return fmt.Sprintf("At minute %d past every hour", m), true
		}
		if len(hourVals) == 1 {
			return fmt.Sprintf("At %02d:%02d", hourVals[0], m), false
		}
		if lo, hi, ok := contiguous(hourVals); ok {
			return fmt.Sprintf("At %d minutes past the hour, between %02d:00 and %02d:59", m, lo, hi), false
		}
		return fmt.Sprintf("At %d minutes past the hour, on hours %s", m, joinInts(hourVals)), false
	}

	if minAll {
		if len(hourVals) == 1 {
			return fmt.Sprintf("Every minute, between %02d:00 and %02d:59", hourVals[0], hourVals[0]), false
		}
		if lo, hi, ok := contiguous(hourVals); ok {
			return fmt.Sprintf("Every minute, between %02d:00 and %02d:59", lo, hi), false
		}
	}

	return fmt.Sprintf("At minutes %s past hours %s", joinInts(minVals), joinInts(hourVals)), false
}

func describeDayClause(s *Schedule, freq bool) string {
	switch {
	case !s.DomRestricted && !s.DowRestricted:
		if freq {
			return ""
		}
		return " every day"
	case s.DomRestricted && !s.DowRestricted:
		return ", " + domPhrase(s)
	case !s.DomRestricted && s.DowRestricted:
		return ", " + dowPhrase(s)
	default:
		return ", " + domPhrase(s) + " or " + dowPhrase(s)
	}
}

func describeMonthClause(s *Schedule) string {
	vals := bitsToList(s.Month, 1, 12)
	if len(vals) == 12 {
		return ""
	}
	if len(vals) == 1 {
		return ", in " + monthFullNames[vals[0]]
	}
	if lo, hi, ok := contiguous(vals); ok {
		return ", in " + monthFullNames[lo] + " through " + monthFullNames[hi]
	}
	names := make([]string, len(vals))
	for i, v := range vals {
		names[i] = monthFullNames[v]
	}
	return ", in " + joinStrings(names)
}

func domPhrase(s *Schedule) string {
	vals := bitsToList(s.Dom, 1, 31)
	if len(vals) == 1 {
		return fmt.Sprintf("on day %d of the month", vals[0])
	}
	if lo, hi, ok := contiguous(vals); ok {
		return fmt.Sprintf("on days %d through %d of the month", lo, hi)
	}
	return "on days " + joinInts(vals) + " of the month"
}

func dowPhrase(s *Schedule) string {
	vals := bitsToList(s.Dow, 0, 6)
	if len(vals) == 1 {
		return "only on " + weekdayNames[vals[0]]
	}
	if lo, hi, ok := contiguous(vals); ok {
		return weekdayNames[lo] + " through " + weekdayNames[hi]
	}
	names := make([]string, len(vals))
	for i, v := range vals {
		names[i] = weekdayNames[v]
	}
	return "on " + joinStrings(names)
}

func bitsToList(bits uint64, lo, hi int) []int {
	var out []int
	for i := lo; i <= hi; i++ {
		if bits&(uint64(1)<<uint(i)) != 0 {
			out = append(out, i)
		}
	}
	return out
}

// arithStep detects a "*/n" field: vals start at lo, are evenly spaced, and
// run up to hi. It returns the spacing.
func arithStep(vals []int, lo, hi int) (int, bool) {
	if len(vals) < 2 || vals[0] != lo {
		return 0, false
	}
	step := vals[1] - vals[0]
	if step < 1 {
		return 0, false
	}
	for i := 1; i < len(vals); i++ {
		if vals[i]-vals[i-1] != step {
			return 0, false
		}
	}
	if vals[len(vals)-1]+step <= hi {
		return 0, false
	}
	return step, true
}

// contiguous reports whether vals is a run of two or more consecutive
// integers, and if so its first and last values.
func contiguous(vals []int) (int, int, bool) {
	if len(vals) < 2 {
		return 0, 0, false
	}
	for i := 1; i < len(vals); i++ {
		if vals[i] != vals[i-1]+1 {
			return 0, 0, false
		}
	}
	return vals[0], vals[len(vals)-1], true
}

func joinInts(vals []int) string {
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = strconv.Itoa(v)
	}
	return joinStrings(strs)
}

// joinStrings gives "a, b and c".
func joinStrings(strs []string) string {
	switch len(strs) {
	case 0:
		return ""
	case 1:
		return strs[0]
	case 2:
		return strs[0] + " and " + strs[1]
	default:
		return strings.Join(strs[:len(strs)-1], ", ") + " and " + strs[len(strs)-1]
	}
}
