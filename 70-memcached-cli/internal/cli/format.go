package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/codingchallenges/ccmc/internal/protocol"
)

// GetOutput is the result of a get, split into what goes to stdout and the
// keys that weren't found.
type GetOutput struct {
	Stdout string // no trailing newline
	Misses []string
}

// FormatGet prints hits in the order the keys were requested, as "key: value"
// or, when quiet, just the value.
func FormatGet(keys []string, vals []protocol.Value, quiet bool) GetOutput {
	hit := make(map[string]protocol.Value, len(vals))
	for _, v := range vals {
		hit[v.Key] = v
	}

	var lines []string
	var misses []string
	seen := make(map[string]bool)
	add := func(v protocol.Value) {
		seen[v.Key] = true
		if quiet {
			lines = append(lines, string(v.Data))
		} else {
			lines = append(lines, formatHit(v.Key, v))
		}
	}
	for _, k := range keys {
		if seen[k] {
			continue
		}
		v, ok := hit[k]
		if !ok {
			seen[k] = true
			misses = append(misses, k)
			continue
		}
		add(v)
	}
	// A server shouldn't send keys we didn't ask for, but show them if it does.
	for _, v := range vals {
		if !seen[v.Key] {
			add(v)
		}
	}

	return GetOutput{Stdout: strings.Join(lines, "\n"), Misses: misses}
}

func formatHit(key string, v protocol.Value) string {
	var b strings.Builder
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(string(v.Data))
	if v.HasCAS {
		b.WriteString("  (flags=")
		b.WriteString(strconv.FormatUint(uint64(v.Flags), 10))
		b.WriteString(" cas=")
		b.WriteString(strconv.FormatUint(v.CAS, 10))
		b.WriteString(")")
	}
	return b.String()
}

// FormatStore describes the result of a storage command.
func FormatStore(status protocol.StoreStatus) string {
	switch status {
	case protocol.Stored:
		return "STORED"
	case protocol.NotStored:
		return "NOT_STORED"
	case protocol.Exists:
		return "EXISTS"
	case protocol.NotFound:
		return "NOT_FOUND"
	case protocol.NoReply:
		return "(noreply: not waiting for acknowledgement)"
	default:
		return "UNKNOWN"
	}
}

// FormatDelete describes the result of a delete.
func FormatDelete(deleted bool) string {
	if deleted {
		return "DELETED"
	}
	return "NOT_FOUND"
}

// FormatNumeric describes the result of incr or decr.
func FormatNumeric(value uint64, found bool) string {
	if !found {
		return "NOT_FOUND"
	}
	return strconv.FormatUint(value, 10)
}

// FormatStats lays stats out in two aligned columns, sorted by name.
func FormatStats(stats []protocol.Stat) string {
	if len(stats) == 0 {
		return "(no stats)"
	}
	sorted := append([]protocol.Stat(nil), stats...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	width := 0
	for _, s := range sorted {
		if len(s.Name) > width {
			width = len(s.Name)
		}
	}
	var b strings.Builder
	for i, s := range sorted {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%-*s  %s", width, s.Name, s.Value)
	}
	return b.String()
}
