// Package lang describes each language's comment and string syntax as data, so
// adding a language only means adding an entry to Registry.
package lang

// SpecialKind marks formats that don't fit the usual comments-and-strings model.
type SpecialKind int

const (
	// None uses the standard comment/string state machine.
	None SpecialKind = iota
	// PlainText has no comment syntax: every non-blank line is "code".
	PlainText
	// Markdown treats fenced ``` code blocks as code and prose as comments.
	Markdown
)

// LanguageDef describes one language's syntax for the classifier in package
// count.
type LanguageDef struct {
	Name          string
	Extensions    []string // lower case, with the dot
	Filenames     []string // exact names such as "Makefile"
	LineComments  []string
	BlockComments [][2]string // start and end markers
	Strings       [][2]string // opening and closing quotes
	NestedBlocks  bool        // block comments nest, as in Rust
	Special       SpecialKind
}
