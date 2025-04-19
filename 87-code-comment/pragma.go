package ccomment

import "strings"

// Pragma comments (noqa, //go:build, eslint-disable and so on) are kept because
// tools read them. -keep-pragma (on by default) controls this.

// bodyAfter strips opener and the whitespace after it, e.g. "#  noqa: E501"
// becomes "noqa: E501".
func bodyAfter(text, opener string) string {
	t := strings.TrimPrefix(text, opener)
	return strings.TrimLeft(t, " \t")
}

func pyIsPragma(s Span) bool {
	if s.Kind != LineComment {
		return false
	}
	t := s.Text
	if strings.HasPrefix(t, "#!") { // shebang
		return true
	}
	body := bodyAfter(t, "#")
	for _, p := range []string{"noqa", "type:", "pylint:", "pragma:",
		"fmt:", "isort:", "mypy:", "coding:", "-*-"} {
		if strings.HasPrefix(body, p) {
			return true
		}
	}
	return false
}

func goIsPragma(s Span) bool {
	if s.Kind != LineComment {
		return false
	}
	t := s.Text
	// Directives have no space after the slashes.
	if strings.HasPrefix(t, "//go:") || strings.HasPrefix(t, "//line ") ||
		strings.HasPrefix(t, "//export ") || strings.HasPrefix(t, "//nolint") {
		return true
	}
	// Old-style build constraint: "// +build linux".
	return strings.HasPrefix(bodyAfter(t, "//"), "+build")
}

func jsIsPragma(s Span) bool {
	if s.Kind != LineComment {
		return false
	}
	body := bodyAfter(s.Text, "//")
	for _, p := range []string{"eslint", "@ts-", "prettier", "sourceMappingURL",
		"webpack", "@flow", "istanbul", "jshint", "globals"} {
		if strings.HasPrefix(body, p) {
			return true
		}
	}
	return false
}

func cIsPragma(s Span) bool {
	if s.Kind != LineComment {
		return false
	}
	body := bodyAfter(s.Text, "//")
	for _, p := range []string{"NOLINT", "clang-format", "IWYU pragma"} {
		if strings.HasPrefix(body, p) {
			return true
		}
	}
	return false
}
