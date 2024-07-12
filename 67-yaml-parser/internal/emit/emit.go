// Package emit writes a Value tree as JSON. Map keys are sorted so output is
// stable. JSON has no infinity or NaN, so those become the strings
// "Infinity", "-Infinity" and "NaN".
package emit

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"yamlcc/internal/value"
)

// Marshal renders v as JSON with indent spaces per level; indent <= 0 gives
// compact output. There is no trailing newline.
func Marshal(v value.Value, indent int) []byte {
	e := &emitter{indent: indent}
	var b strings.Builder
	e.write(&b, v, 0)
	return []byte(b.String())
}

type emitter struct {
	indent int
}

func (e *emitter) nl() string {
	if e.indent <= 0 {
		return ""
	}
	return "\n"
}

func (e *emitter) pad(depth int) string {
	if e.indent <= 0 {
		return ""
	}
	return strings.Repeat(" ", e.indent*depth)
}

func (e *emitter) colonSep() string {
	if e.indent <= 0 {
		return ":"
	}
	return ": "
}

func (e *emitter) write(b *strings.Builder, v value.Value, depth int) {
	switch v.Kind {
	case value.Null:
		b.WriteString("null")
	case value.Bool:
		if v.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case value.Int:
		b.WriteString(strconv.FormatInt(v.Int, 10))
	case value.Float:
		b.WriteString(formatFloat(v.Float))
	case value.Str:
		b.WriteString(jsonString(v.Str))
	case value.Seq:
		e.writeSeq(b, v.Seq, depth)
	case value.Map:
		e.writeMap(b, v.Items, depth)
	}
}

func (e *emitter) writeSeq(b *strings.Builder, items []value.Value, depth int) {
	if len(items) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[")
	b.WriteString(e.nl())
	for i, it := range items {
		b.WriteString(e.pad(depth + 1))
		e.write(b, it, depth+1)
		if i < len(items)-1 {
			b.WriteString(",")
		}
		b.WriteString(e.nl())
	}
	b.WriteString(e.pad(depth))
	b.WriteString("]")
}

func (e *emitter) writeMap(b *strings.Builder, items []value.KV, depth int) {
	if len(items) == 0 {
		b.WriteString("{}")
		return
	}
	sorted := make([]value.KV, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	b.WriteString("{")
	b.WriteString(e.nl())
	for i, kv := range sorted {
		b.WriteString(e.pad(depth + 1))
		b.WriteString(jsonString(kv.Key))
		b.WriteString(e.colonSep())
		e.write(b, kv.Val, depth+1)
		if i < len(sorted)-1 {
			b.WriteString(",")
		}
		b.WriteString(e.nl())
	}
	b.WriteString(e.pad(depth))
	b.WriteString("}")
}

func formatFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return `"NaN"`
	case math.IsInf(f, 1):
		return `"Infinity"`
	case math.IsInf(f, -1):
		return `"-Infinity"`
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // can't fail for a string
	return strings.TrimRight(buf.String(), "\n")
}
