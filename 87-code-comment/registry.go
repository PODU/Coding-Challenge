package ccomment

import (
	"path/filepath"
	"strings"
)

// basicStrip is Strip for languages where pragmas are the only comments
// worth keeping (everything except Go).
func basicStrip(s Stripper, src []byte, opt Options, isPragma func(Span) bool) ([]byte, Stats, error) {
	spans, err := s.Scan(src)
	if err != nil {
		return nil, Stats{}, err
	}
	keep := func(sp Span) bool { return opt.KeepPragma && isPragma(sp) }
	out, st := stripWith(src, spans, opt, keep)
	return out, st, nil
}

func cFamily(name string, regex, lineCont bool, pragma func(Span) bool) cStripper {
	return cStripper{cfg: cConfig{name: name, regex: regex, lineCont: lineCont,
		backtick: btNone, pragma: pragma}}
}

func jsStripper(name string) cStripper {
	return cStripper{cfg: cConfig{name: name, regex: true, backtick: btTemplate, pragma: jsIsPragma}}
}

// For returns the Stripper for a language name such as "python" or "ts".
func For(lang string) (Stripper, bool) {
	switch strings.ToLower(lang) {
	case "python", "py":
		return pythonStripper{}, true
	case "js", "javascript", "node":
		return jsStripper("js"), true
	case "ts", "typescript":
		return jsStripper("ts"), true
	case "c":
		return cFamily("c", false, true, cIsPragma), true
	case "cpp", "c++", "cc", "cxx":
		return cFamily("cpp", false, true, cIsPragma), true
	case "go", "golang":
		return newGoStripper(), true
	}
	return nil, false
}

// ForFile picks a Stripper from a file's extension.
func ForFile(path string) (Stripper, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py", ".pyw":
		return For("python")
	case ".js", ".jsx", ".mjs", ".cjs":
		return For("js")
	case ".ts", ".tsx":
		return For("ts")
	case ".c", ".h":
		return For("c")
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx":
		return For("cpp")
	case ".go":
		return For("go")
	}
	return nil, false
}
