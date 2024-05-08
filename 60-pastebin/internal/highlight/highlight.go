// Package highlight validates the language chosen for a paste and maps it to
// the "language-X" CSS class the view template puts on the <code> element.
// No highlighting happens on the server; content is always HTML-escaped.
package highlight

import "strings"

// supported maps language tokens to display names. "" is plain text.
var supported = map[string]string{
	"":           "Plain text",
	"go":         "Go",
	"python":     "Python",
	"javascript": "JavaScript",
	"typescript": "TypeScript",
	"java":       "Java",
	"c":          "C",
	"cpp":        "C++",
	"csharp":     "C#",
	"rust":       "Rust",
	"ruby":       "Ruby",
	"php":        "PHP",
	"shell":      "Shell",
	"sql":        "SQL",
	"html":       "HTML",
	"css":        "CSS",
	"json":       "JSON",
	"yaml":       "YAML",
	"markdown":   "Markdown",
}

// Language is one entry in the UI's language dropdown.
type Language struct {
	Token string
	Label string
}

// Normalize lower-cases and trims lang. Unknown languages return "" and false.
func Normalize(lang string) (token string, ok bool) {
	t := strings.ToLower(strings.TrimSpace(lang))
	if _, found := supported[t]; found {
		return t, true
	}
	return "", false
}

// CSSClass takes a token already passed through Normalize.
func CSSClass(token string) string {
	if token == "" {
		return "language-none"
	}
	return "language-" + token
}

// Languages returns the languages in dropdown order.
func Languages() []Language {
	order := []string{
		"", "go", "python", "javascript", "typescript", "java", "c", "cpp",
		"csharp", "rust", "ruby", "php", "shell", "sql", "html", "css", "json",
		"yaml", "markdown",
	}
	out := make([]Language, 0, len(order))
	for _, t := range order {
		out = append(out, Language{Token: t, Label: supported[t]})
	}
	return out
}
