package lang

import (
	"path/filepath"
	"strings"
)

var (
	byExt      = map[string]*LanguageDef{}
	byFilename = map[string]*LanguageDef{}
)

func init() {
	for i := range Registry {
		d := &Registry[i]
		for _, e := range d.Extensions {
			byExt[strings.ToLower(e)] = d
		}
		for _, f := range d.Filenames {
			byFilename[f] = d
		}
	}
}

// Detect looks a path up by exact filename (e.g. "Makefile"), then by
// extension, ignoring case.
func Detect(path string) (*LanguageDef, bool) {
	base := filepath.Base(path)
	if d, ok := byFilename[base]; ok {
		return d, true
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return nil, false
	}
	if d, ok := byExt[ext]; ok {
		return d, true
	}
	return nil, false
}

// ByName returns the language definition with the given name (case-insensitive).
func ByName(name string) (*LanguageDef, bool) {
	for i := range Registry {
		if strings.EqualFold(Registry[i].Name, name) {
			return &Registry[i], true
		}
	}
	return nil, false
}

// UnknownDef is the plain-text language used for unrecognised files with
// --include-unknown.
func UnknownDef() *LanguageDef {
	return &LanguageDef{Name: "(unknown)", Special: PlainText}
}
