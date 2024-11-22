// Package template fills in the {{ Title }} and {{ Content }} placeholders in
// a theme. Spacing inside the braces doesn't matter, and unknown placeholders
// are left as they are.
package template

import "regexp"

var tagRe = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// Render substitutes vars into tmpl.
func Render(tmpl string, vars map[string]string) string {
	return tagRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := tagRe.FindStringSubmatch(match)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}

// HasTag reports whether tmpl has a {{ name }} placeholder.
func HasTag(tmpl, name string) bool {
	for _, m := range tagRe.FindAllStringSubmatch(tmpl, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}
