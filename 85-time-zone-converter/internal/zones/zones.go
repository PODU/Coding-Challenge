// Package zones looks up timezones by IANA name, city or abbreviation.
package zones

import (
	"fmt"
	"sort"
	"strings"
	"time"

	// Use the bundled tz database so every host gives the same answers.
	_ "time/tzdata"
)

type Zone struct {
	Name string
	Loc  *time.Location
}

// Resolution is what a token resolved to, and how.
type Resolution struct {
	Token string
	Name  string // canonical IANA name
	Loc   *time.Location
	Via   string // "iana", "city", or "abbrev"
	Note  string // e.g. which zone an ambiguous abbreviation was taken to mean
}

func (r Resolution) Zone() Zone { return Zone{Name: r.Name, Loc: r.Loc} }

// abbrev is the zone an abbreviation stands for. Many abbreviations are
// ambiguous, so note says which meaning was chosen.
type abbrev struct {
	name string
	note string
}

// cityAliases maps a normalized city/region token to an IANA zone name.
var cityAliases = map[string]string{
	"utc":           "UTC",
	"new york":      "America/New_York",
	"nyc":           "America/New_York",
	"los angeles":   "America/Los_Angeles",
	"la":            "America/Los_Angeles",
	"san francisco": "America/Los_Angeles",
	"seattle":       "America/Los_Angeles",
	"chicago":       "America/Chicago",
	"denver":        "America/Denver",
	"phoenix":       "America/Phoenix",
	"anchorage":     "America/Anchorage",
	"toronto":       "America/Toronto",
	"vancouver":     "America/Vancouver",
	"mexico city":   "America/Mexico_City",
	"sao paulo":     "America/Sao_Paulo",
	"buenos aires":  "America/Argentina/Buenos_Aires",
	"london":        "Europe/London",
	"dublin":        "Europe/Dublin",
	"lisbon":        "Europe/Lisbon",
	"paris":         "Europe/Paris",
	"berlin":        "Europe/Berlin",
	"madrid":        "Europe/Madrid",
	"rome":          "Europe/Rome",
	"amsterdam":     "Europe/Amsterdam",
	"brussels":      "Europe/Brussels",
	"zurich":        "Europe/Zurich",
	"vienna":        "Europe/Vienna",
	"athens":        "Europe/Athens",
	"moscow":        "Europe/Moscow",
	"istanbul":      "Europe/Istanbul",
	"dubai":         "Asia/Dubai",
	"tehran":        "Asia/Tehran",
	"karachi":       "Asia/Karachi",
	"mumbai":        "Asia/Kolkata",
	"delhi":         "Asia/Kolkata",
	"new delhi":     "Asia/Kolkata",
	"kolkata":       "Asia/Kolkata",
	"calcutta":      "Asia/Kolkata",
	"kathmandu":     "Asia/Kathmandu",
	"dhaka":         "Asia/Dhaka",
	"bangkok":       "Asia/Bangkok",
	"jakarta":       "Asia/Jakarta",
	"singapore":     "Asia/Singapore",
	"hong kong":     "Asia/Hong_Kong",
	"shanghai":      "Asia/Shanghai",
	"beijing":       "Asia/Shanghai",
	"taipei":        "Asia/Taipei",
	"tokyo":         "Asia/Tokyo",
	"seoul":         "Asia/Seoul",
	"sydney":        "Australia/Sydney",
	"melbourne":     "Australia/Melbourne",
	"brisbane":      "Australia/Brisbane",
	"perth":         "Australia/Perth",
	"adelaide":      "Australia/Adelaide",
	"auckland":      "Pacific/Auckland",
	"honolulu":      "Pacific/Honolulu",
}

// abbrevAliases maps an upper-cased abbreviation to a canonical IANA zone.
var abbrevAliases = map[string]abbrev{
	"UTC":  {"UTC", ""},
	"GMT":  {"Etc/GMT", ""},
	"EST":  {"America/New_York", "EST also denotes Australian Eastern Standard Time; chose US Eastern."},
	"EDT":  {"America/New_York", ""},
	"CST":  {"America/Chicago", "CST is ambiguous (US Central / China / Cuba); chose US Central. Use an IANA name to override."},
	"CDT":  {"America/Chicago", ""},
	"MST":  {"America/Denver", ""},
	"MDT":  {"America/Denver", ""},
	"PST":  {"America/Los_Angeles", "PST also denotes Philippine Standard Time; chose US Pacific."},
	"PDT":  {"America/Los_Angeles", ""},
	"AKST": {"America/Anchorage", ""},
	"AKDT": {"America/Anchorage", ""},
	"HST":  {"Pacific/Honolulu", ""},
	"AST":  {"America/Halifax", "AST is ambiguous (Atlantic / Arabia); chose Atlantic."},
	"ADT":  {"America/Halifax", ""},
	"NST":  {"America/St_Johns", ""},
	"BST":  {"Europe/London", "BST is ambiguous (British Summer Time / Bangladesh); chose British."},
	"WET":  {"Europe/Lisbon", ""},
	"WEST": {"Europe/Lisbon", ""},
	"CET":  {"Europe/Paris", ""},
	"CEST": {"Europe/Paris", ""},
	"EET":  {"Europe/Athens", ""},
	"EEST": {"Europe/Athens", ""},
	"MSK":  {"Europe/Moscow", ""},
	"IST":  {"Asia/Kolkata", "IST is ambiguous (India / Ireland / Israel); chose India. Use an IANA name to override."},
	"PKT":  {"Asia/Karachi", ""},
	"GST":  {"Asia/Dubai", ""},
	"ICT":  {"Asia/Bangkok", ""},
	"SGT":  {"Asia/Singapore", ""},
	"HKT":  {"Asia/Hong_Kong", ""},
	"JST":  {"Asia/Tokyo", ""},
	"KST":  {"Asia/Seoul", ""},
	"AEST": {"Australia/Sydney", ""},
	"AEDT": {"Australia/Sydney", ""},
	"ACST": {"Australia/Adelaide", ""},
	"AWST": {"Australia/Perth", ""},
	"NZST": {"Pacific/Auckland", ""},
	"NZDT": {"Pacific/Auckland", ""},
}

// normalize lower-cases, converts underscores to spaces, trims, and collapses
// internal whitespace so "New_York", "new york ", and "NEW  YORK" all match.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

// Resolve maps a token to a *time.Location. Resolution order:
//  1. region IANA names containing "/" (e.g. America/New_York),
//  2. city aliases,
//  3. abbreviations,
//  4. bare IANA names (e.g. UTC).
//
// Abbreviations are checked before bare IANA names so that "EST" resolves to
// America/New_York rather than the fixed-offset "EST" zone in the database.
func Resolve(token string) (Resolution, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return Resolution{}, fmt.Errorf("empty zone token")
	}

	// 1. Region IANA name.
	if strings.Contains(trimmed, "/") {
		if loc, err := time.LoadLocation(trimmed); err == nil {
			return Resolution{Token: token, Name: loc.String(), Loc: loc, Via: "iana"}, nil
		}
		return Resolution{}, unknownZone(token)
	}

	// 2. City alias.
	if name, ok := cityAliases[normalize(trimmed)]; ok {
		loc, err := time.LoadLocation(name)
		if err != nil {
			return Resolution{}, fmt.Errorf("alias %q maps to %q which failed to load: %w", token, name, err)
		}
		return Resolution{Token: token, Name: name, Loc: loc, Via: "city"}, nil
	}

	// 3. Abbreviation.
	if a, ok := abbrevAliases[strings.ToUpper(trimmed)]; ok {
		loc, err := time.LoadLocation(a.name)
		if err != nil {
			return Resolution{}, fmt.Errorf("abbreviation %q maps to %q which failed to load: %w", token, a.name, err)
		}
		return Resolution{Token: token, Name: a.name, Loc: loc, Via: "abbrev", Note: a.note}, nil
	}

	// 4. Bare IANA name (UTC, Local-style names, etc.).
	if loc, err := time.LoadLocation(trimmed); err == nil {
		return Resolution{Token: token, Name: loc.String(), Loc: loc, Via: "iana"}, nil
	}

	return Resolution{}, unknownZone(token)
}

// unknownZone builds an actionable error including nearest suggestions.
func unknownZone(token string) error {
	sugg := Suggest(token, 5)
	if len(sugg) == 0 {
		return fmt.Errorf("unknown zone %q; run with --list to see known zones and aliases", token)
	}
	return fmt.Errorf("unknown zone %q; did you mean: %s? (run --list for all)", token, strings.Join(sugg, ", "))
}

// Suggest returns up to n alias/abbreviation keys nearest to the token.
func Suggest(token string, n int) []string {
	q := normalize(token)
	type cand struct {
		key  string
		dist int
	}
	var cands []cand
	seen := map[string]bool{}
	add := func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		d := levenshtein(q, normalize(key))
		// Reward substring matches.
		if strings.Contains(normalize(key), q) || strings.Contains(q, normalize(key)) {
			d = 0
		}
		cands = append(cands, cand{key, d})
	}
	for k := range cityAliases {
		add(k)
	}
	for k := range abbrevAliases {
		add(k)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].key < cands[j].key
	})
	var out []string
	for _, c := range cands {
		if c.dist > 4 {
			break
		}
		out = append(out, c.key)
		if len(out) >= n {
			break
		}
	}
	return out
}

// ListAliases returns the sorted city aliases and abbreviations for --list.
func ListAliases() (cities []string, abbrevs []string) {
	for k, v := range cityAliases {
		cities = append(cities, fmt.Sprintf("%-14s -> %s", k, v))
	}
	for k, v := range abbrevAliases {
		abbrevs = append(abbrevs, fmt.Sprintf("%-5s -> %s", k, v.name))
	}
	sort.Strings(cities)
	sort.Strings(abbrevs)
	return cities, abbrevs
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
