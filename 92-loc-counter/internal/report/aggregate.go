// Package report totals the counts by language and prints them as a table,
// JSON, or a COCOMO estimate.
package report

import (
	"sort"
	"strings"

	"loc/internal/count"
)

// LangTotal is the totals for one language, or for everything.
type LangTotal struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
	Blank    int    `json:"blank"`
	Comment  int    `json:"comment"`
	Code     int    `json:"code"`
}

// Report holds the per-language totals, the grand total, and the per-file counts
// for --by-file.
type Report struct {
	Languages []LangTotal
	Total     LangTotal
	Files     []count.FileCount
}

// Aggregate totals files by language. Rows are sorted by sortKey: "code"
// (default), "lines", "comments" or "blanks" descending, or "name" ascending,
// with ties broken by name.
func Aggregate(files []count.FileCount, sortKey string) Report {
	byLang := map[string]*LangTotal{}
	var order []string
	for _, fc := range files {
		lt, ok := byLang[fc.Lang]
		if !ok {
			lt = &LangTotal{Language: fc.Lang}
			byLang[fc.Lang] = lt
			order = append(order, fc.Lang)
		}
		lt.Files++
		lt.Lines += fc.Lines
		lt.Blank += fc.Blank
		lt.Comment += fc.Comment
		lt.Code += fc.Code
	}

	rep := Report{Files: files}
	for _, name := range order {
		lt := byLang[name]
		rep.Languages = append(rep.Languages, *lt)
		rep.Total.Files += lt.Files
		rep.Total.Lines += lt.Lines
		rep.Total.Blank += lt.Blank
		rep.Total.Comment += lt.Comment
		rep.Total.Code += lt.Code
	}
	rep.Total.Language = "Total"

	sortLangs(rep.Languages, sortKey)
	return rep
}

func sortLangs(langs []LangTotal, key string) {
	less := func(i, j int) bool {
		a, b := langs[i], langs[j]
		switch strings.ToLower(key) {
		case "name":
			return a.Language < b.Language
		case "lines":
			if a.Lines != b.Lines {
				return a.Lines > b.Lines
			}
		case "comments":
			if a.Comment != b.Comment {
				return a.Comment > b.Comment
			}
		case "blanks":
			if a.Blank != b.Blank {
				return a.Blank > b.Blank
			}
		default: // "code"
			if a.Code != b.Code {
				return a.Code > b.Code
			}
		}
		return a.Language < b.Language
	}
	sort.SliceStable(langs, less)
}
