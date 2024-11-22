// Package markdown renders the subset of Markdown ccsg needs: # headings,
// paragraphs, lists, fenced code, blockquotes, horizontal rules, **bold**,
// *italic*, `code` and [text](url) links. Text is HTML-escaped.
package markdown

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	headingRe     = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	h1Re          = regexp.MustCompile(`^#\s+(.+)$`)
	ulRe          = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)
	olRe          = regexp.MustCompile(`^\s*\d+\.\s+(.*)$`)
	inlineCodeRe  = regexp.MustCompile("`[^`]+`")
	linkRe        = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	boldRe        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRe      = regexp.MustCompile(`\*([^*]+)\*`)
	placeholderRe = regexp.MustCompile("\x00(\\d+)\x00")
)

// Title returns the first H1 heading in src, or "".
func Title(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	for _, line := range strings.Split(src, "\n") {
		if m := h1Re.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func Render(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")

	var out strings.Builder
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "```"):
			i = renderFence(&out, lines, i)
		case trimmed == "":
			i++
		case trimmed == "---" || trimmed == "***" || trimmed == "___":
			out.WriteString("<hr>\n")
			i++
		case headingRe.MatchString(line):
			m := headingRe.FindStringSubmatch(line)
			tag := "h" + strconv.Itoa(len(m[1]))
			out.WriteString("<" + tag + ">" + renderInline(strings.TrimSpace(m[2])) + "</" + tag + ">\n")
			i++
		case strings.HasPrefix(trimmed, ">"):
			i = renderBlockquote(&out, lines, i)
		case ulRe.MatchString(line):
			i = renderList(&out, lines, i, ulRe, "ul")
		case olRe.MatchString(line):
			i = renderList(&out, lines, i, olRe, "ol")
		default:
			i = renderParagraph(&out, lines, i)
		}
	}
	return out.String()
}

func renderFence(out *strings.Builder, lines []string, i int) int {
	lang := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), "```"))
	i++
	var code []string
	for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
		code = append(code, lines[i])
		i++
	}
	if i < len(lines) {
		i++ // closing fence
	}
	out.WriteString("<pre><code")
	if lang != "" {
		out.WriteString(` class="language-` + html.EscapeString(lang) + `"`)
	}
	out.WriteString(">")
	out.WriteString(html.EscapeString(strings.Join(code, "\n")))
	if len(code) > 0 {
		out.WriteString("\n")
	}
	out.WriteString("</code></pre>\n")
	return i
}

func renderBlockquote(out *strings.Builder, lines []string, i int) int {
	var quote []string
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
		q := strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")
		quote = append(quote, strings.TrimPrefix(q, " "))
		i++
	}
	out.WriteString("<blockquote>" + renderInline(strings.Join(quote, " ")) + "</blockquote>\n")
	return i
}

func renderList(out *strings.Builder, lines []string, i int, re *regexp.Regexp, tag string) int {
	out.WriteString("<" + tag + ">\n")
	for i < len(lines) && re.MatchString(lines[i]) {
		item := re.FindStringSubmatch(lines[i])[1]
		out.WriteString("<li>" + renderInline(strings.TrimSpace(item)) + "</li>\n")
		i++
	}
	out.WriteString("</" + tag + ">\n")
	return i
}

func renderParagraph(out *strings.Builder, lines []string, i int) int {
	var para []string
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" || t == "---" || t == "***" || t == "___" ||
			strings.HasPrefix(t, "```") || strings.HasPrefix(t, ">") ||
			headingRe.MatchString(lines[i]) || ulRe.MatchString(lines[i]) || olRe.MatchString(lines[i]) {
			break
		}
		para = append(para, t)
		i++
	}
	if len(para) > 0 {
		out.WriteString("<p>" + renderInline(strings.Join(para, " ")) + "</p>\n")
	}
	return i
}

// renderInline handles inline formatting. Code spans are swapped out for
// placeholders first so that nothing inside them gets formatted.
func renderInline(s string) string {
	var codes []string
	s = inlineCodeRe.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, "<code>"+html.EscapeString(m[1:len(m)-1])+"</code>")
		return "\x00" + strconv.Itoa(len(codes)-1) + "\x00"
	})

	s = html.EscapeString(s)
	s = linkRe.ReplaceAllString(s, `<a href="$2">$1</a>`)
	s = boldRe.ReplaceAllString(s, `<strong>$1</strong>`)
	s = italicRe.ReplaceAllString(s, `<em>$1</em>`)

	s = placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
		idx, _ := strconv.Atoi(m[1 : len(m)-1])
		return codes[idx]
	})
	return s
}

// TitleCase turns a file name like "my-first_post" into "My First Post".
func TitleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
