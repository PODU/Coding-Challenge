// Package scaffold creates new sites, themes and pages.
package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ccsg/internal/markdown"
)

// ErrExists means the thing to be created is already there.
var ErrExists = errors.New("target already exists")

const defaultTheme = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ Title }}</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 42rem; margin: 2rem auto; padding: 0 1rem; line-height: 1.6; }
    pre { background: #f4f4f4; padding: 1rem; overflow-x: auto; }
    code { background: #f4f4f4; padding: 0.1rem 0.3rem; }
    pre code { background: none; padding: 0; }
    blockquote { border-left: 4px solid #ddd; margin: 0; padding-left: 1rem; color: #555; }
  </style>
</head>
<body>
  <main>
{{ Content }}
  </main>
</body>
</html>
`

const indexContent = `# Welcome to your new site

This is your homepage, written in **Markdown**.

Edit ` + "`content/index.md`" + ` and run ` + "`ccsg build`" + ` to regenerate the site.
`

// NewSite creates a site with a starter homepage, the default theme and a
// config file. An empty existing directory is fine; anything else at name is
// ErrExists.
func NewSite(name string) error {
	if info, err := os.Stat(name); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%q: %w", name, ErrExists)
		}
		entries, err := os.ReadDir(name)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("directory %q exists and is not empty: %w", name, ErrExists)
		}
	}

	files := map[string]string{
		filepath.Join(name, "content", "index.md"):             indexContent,
		filepath.Join(name, "themes", "default", "index.html"): defaultTheme,
		filepath.Join(name, "config"):                          "theme = default\n",
	}
	for path, content := range files {
		if err := writeFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

const themeTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ Title }}</title>
</head>
<body>
  <article>
{{ Content }}
  </article>
</body>
</html>
`

// NewTheme creates themes/<name>/index.html.
func NewTheme(root, name string) error {
	dir := filepath.Join(root, "themes", name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("theme %q: %w", name, ErrExists)
	}
	return writeFile(filepath.Join(dir, "index.html"), themeTemplate)
}

// NewPage creates content/<name>.md. name may contain directories, e.g.
// "blog/first".
func NewPage(root, name string) (string, error) {
	rel := filepath.FromSlash(name)
	rel = strings.TrimSuffix(rel, ".md")
	target := filepath.Join(root, "content", rel+".md")
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%q: %w", target, ErrExists)
	}
	title := markdown.TitleCase(filepath.Base(rel))
	body := fmt.Sprintf("# %s\n\nWrite your content here.\n", title)
	if err := writeFile(target, body); err != nil {
		return "", err
	}
	return target, nil
}

// FindSiteRoot returns the nearest directory at or above start that IsSite.
func FindSiteRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if IsSite(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside a ccsg site (no ancestor directory has both content/ and themes/)")
		}
		dir = parent
	}
}

// IsSite reports whether dir contains both content/ and themes/ directories.
func IsSite(dir string) bool {
	c, err1 := os.Stat(filepath.Join(dir, "content"))
	t, err2 := os.Stat(filepath.Join(dir, "themes"))
	return err1 == nil && c.IsDir() && err2 == nil && t.IsDir()
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
