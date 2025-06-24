package lang

// Registry lists the supported languages.
//
// The classifier treats backtick strings as raw (no backslash escapes). Python
// triple-quoted strings are listed as block comments, which counts docstrings
// as comments at the cost of also counting other triple-quoted strings.
var Registry = []LanguageDef{
	{
		Name:          "Go",
		Extensions:    []string{".go"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"`", "`"}, {"'", "'"}},
	},
	{
		Name:          "Python",
		Extensions:    []string{".py", ".pyw"},
		LineComments:  []string{"#"},
		BlockComments: [][2]string{{`"""`, `"""`}, {"'''", "'''"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "JavaScript",
		Extensions:    []string{".js", ".jsx", ".mjs", ".cjs"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}, {"`", "`"}},
	},
	{
		Name:          "TypeScript",
		Extensions:    []string{".ts", ".tsx"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}, {"`", "`"}},
	},
	{
		Name:          "Java",
		Extensions:    []string{".java"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "C",
		Extensions:    []string{".c", ".h"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "C++",
		Extensions:    []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "C#",
		Extensions:    []string{".cs"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "Rust",
		Extensions:    []string{".rs"},
		LineComments:  []string{"//"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}},
		NestedBlocks:  true,
	},
	{
		Name:         "Ruby",
		Extensions:   []string{".rb"},
		Filenames:    []string{"Rakefile", "Gemfile"},
		LineComments: []string{"#"},
		// =begin and =end must start a line in Ruby; treating them as an
		// ordinary block pair is close enough in practice.
		BlockComments: [][2]string{{"=begin", "=end"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:         "Shell",
		Extensions:   []string{".sh", ".bash", ".zsh"},
		Filenames:    []string{".bashrc", ".zshrc"},
		LineComments: []string{"#"},
		Strings:      [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "HTML",
		Extensions:    []string{".html", ".htm"},
		BlockComments: [][2]string{{"<!--", "-->"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "CSS",
		Extensions:    []string{".css"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:          "SQL",
		Extensions:    []string{".sql"},
		LineComments:  []string{"--"},
		BlockComments: [][2]string{{"/*", "*/"}},
		Strings:       [][2]string{{`'`, `'`}},
	},
	{
		Name:         "YAML",
		Extensions:   []string{".yaml", ".yml"},
		LineComments: []string{"#"},
		Strings:      [][2]string{{`"`, `"`}, {"'", "'"}},
	},
	{
		Name:       "JSON",
		Extensions: []string{".json"},
		Strings:    [][2]string{{`"`, `"`}},
	},
	{
		Name:       "Text",
		Extensions: []string{".txt", ".text"},
		Special:    PlainText,
	},
	{
		Name:       "Markdown",
		Extensions: []string{".md", ".markdown"},
		Special:    Markdown,
	},
	{
		Name:         "Makefile",
		Filenames:    []string{"Makefile", "makefile", "GNUmakefile"},
		Extensions:   []string{".mk"},
		LineComments: []string{"#"},
	},
	{
		Name:         "Dockerfile",
		Filenames:    []string{"Dockerfile"},
		LineComments: []string{"#"},
		Strings:      [][2]string{{`"`, `"`}, {"'", "'"}},
	},
}
