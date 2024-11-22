// Package builder renders content/**.md through the active theme into
// public/.
package builder

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ccsg/internal/markdown"
	"ccsg/internal/template"
)

// Result summarises a build.
type Result struct {
	Theme             string   // active theme name
	Pages             []string // e.g. "public/index.html", relative to root
	MissingContentTag bool     // the theme has no {{ Content }}
}

// Build regenerates public/ from scratch, so pages whose source was deleted
// don't linger.
func Build(root string) (Result, error) {
	var res Result
	res.Theme = ActiveTheme(root)

	themePath := filepath.Join(root, "themes", res.Theme, "index.html")
	tmplBytes, err := os.ReadFile(themePath)
	if err != nil {
		return res, fmt.Errorf("reading theme %q: %w", res.Theme, err)
	}
	tmpl := string(tmplBytes)
	res.MissingContentTag = !template.HasTag(tmpl, "Content")

	contentDir := filepath.Join(root, "content")
	publicDir := filepath.Join(root, "public")
	if err := os.RemoveAll(publicDir); err != nil {
		return res, err
	}

	walkErr := filepath.WalkDir(contentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(contentDir, path)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		title := markdown.Title(string(src))
		if title == "" {
			base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
			title = markdown.TitleCase(base)
		}
		page := template.Render(tmpl, map[string]string{
			"Title":   title,
			"Content": markdown.Render(string(src)),
		})

		outRel := strings.TrimSuffix(rel, filepath.Ext(rel)) + ".html"
		outPath := filepath.Join(publicDir, outRel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
			return err
		}
		res.Pages = append(res.Pages, filepath.ToSlash(filepath.Join("public", outRel)))
		return nil
	})
	return res, walkErr
}

// ActiveTheme returns the theme named in the site's config file, or
// "default".
func ActiveTheme(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "config"))
	if err != nil {
		return "default"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "theme") {
			if _, value, ok := strings.Cut(line, "="); ok {
				if v := strings.TrimSpace(value); v != "" {
					return v
				}
			}
		}
	}
	return "default"
}
