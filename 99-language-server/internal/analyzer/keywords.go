package analyzer

import "greetls/internal/lsp"

// Keyword is a .greet keyword or attribute, as shown in hover and completion.
type Keyword struct {
	Label  string
	Detail string
	Doc    string // Markdown documentation
}

// Directives that can start a line.
var keywords = []Keyword{
	{
		Label:  "greet",
		Detail: `greet "<text>"`,
		Doc:    "**greet** — sets the greeting text.\n\nRequires a quoted string argument, e.g. `greet \"World\"`.",
	},
	{
		Label:  "lang",
		Detail: "lang <code>",
		Doc:    "**lang** — sets the language code (e.g. `en`, `fr`).\n\nOnly one `lang` directive is allowed per document.",
	},
	{
		Label:  "repeat",
		Detail: "repeat <n>",
		Doc:    "**repeat** — repeats the greeting `n` times.\n\n`n` must be a positive integer.",
	},
}

// Names that can follow '@'.
var attributes = []Keyword{
	{
		Label:  "style",
		Detail: "@style=<value>",
		Doc:    "**style** — greeting style attribute (e.g. `formal`, `casual`).",
	},
}

var keywordIndex = func() map[string]Keyword {
	m := make(map[string]Keyword, len(keywords)+len(attributes))
	for _, k := range keywords {
		m[k.Label] = k
	}
	for _, k := range attributes {
		m[k.Label] = k
	}
	return m
}()

func keywordItems() []lsp.CompletionItem {
	return completionItems(keywords, lsp.CompletionKeyword)
}

func attributeItems() []lsp.CompletionItem {
	return completionItems(attributes, lsp.CompletionProperty)
}

func completionItems(list []Keyword, kind lsp.CompletionItemKind) []lsp.CompletionItem {
	items := make([]lsp.CompletionItem, 0, len(list))
	for _, k := range list {
		items = append(items, lsp.CompletionItem{
			Label:         k.Label,
			Kind:          kind,
			Detail:        k.Detail,
			Documentation: k.Doc,
			InsertText:    k.Label,
		})
	}
	return items
}
