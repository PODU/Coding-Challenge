/**
 * Text helpers for importing a script: rewriting share URLs into something
 * fetchable, cleaning up plain text, and a small Markdown to HTML converter.
 * The fetch itself is in ui/net.ts.
 */

import { normalizeNewlines } from "./script.js";

/** Convert line endings to LF and drop a leading BOM. */
export function fromPlainText(s: string): string {
  return normalizeNewlines(s).replace(/^﻿/, "");
}

/**
 * Turn a GitHub file page URL into the raw file URL:
 *
 *   https://github.com/<owner>/<repo>/blob/<ref>/<path>
 *     -> https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>
 *
 * Anything else is returned unchanged.
 */
export function fromGithubUrl(url: string): string {
  const trimmed = url.trim();
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return trimmed;
  }

  if (parsed.hostname === "raw.githubusercontent.com") return trimmed;

  if (parsed.hostname === "github.com" || parsed.hostname === "www.github.com") {
    const parts = parsed.pathname.split("/").filter((p) => p.length > 0);
    const blobIdx = parts.indexOf("blob");
    if (blobIdx >= 2 && parts.length > blobIdx + 1) {
      const [owner, repo] = parts;
      const rest = parts.slice(blobIdx + 1).join("/");
      return `https://raw.githubusercontent.com/${owner}/${repo}/${rest}`;
    }
  }

  return trimmed;
}

/**
 * Turn a Google Docs URL into its plain-text export URL:
 *
 *   https://docs.google.com/document/d/<id>/edit?...
 *     -> https://docs.google.com/document/d/<id>/export?format=txt
 *
 * URLs without a document id are returned unchanged.
 */
export function fromGoogleDocUrl(url: string): string {
  const trimmed = url.trim();
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    return trimmed;
  }

  if (parsed.hostname !== "docs.google.com") return trimmed;

  const match = parsed.pathname.match(/\/document\/d\/([^/]+)/);
  if (!match || !match[1]) return trimmed;

  return `https://docs.google.com/document/d/${match[1]}/export?format=txt`;
}

/** Apply whichever of the rewrites above matches the URL's host. */
export function resolveImportUrl(url: string): string {
  const trimmed = url.trim();
  let host: string;
  try {
    host = new URL(trimmed).hostname;
  } catch {
    return trimmed;
  }
  if (host.endsWith("github.com")) return fromGithubUrl(trimmed);
  if (host === "docs.google.com") return fromGoogleDocUrl(trimmed);
  return trimmed;
}

const HTML_ESCAPES: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => HTML_ESCAPES[c] ?? c);
}

/** Bold, italic and inline code. The input must already be HTML-escaped. */
function renderInline(escaped: string): string {
  return escaped
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/(^|[^*])\*([^*]+)\*/g, "$1<em>$2</em>")
    .replace(/_([^_]+)_/g, "<em>$1</em>")
    .replace(/`([^`]+)`/g, "<code>$1</code>");
}

/**
 * Very small Markdown to HTML converter: `#` headings, bold, italic, inline
 * code and paragraphs. Anything it can't handle comes out as escaped text.
 */
export function fromMarkdown(md: string): { html: string } {
  try {
    const lines = normalizeNewlines(md).split("\n");
    const out: string[] = [];
    let paragraph: string[] = [];

    const flushParagraph = (): void => {
      if (paragraph.length === 0) return;
      const text = paragraph.join(" ");
      out.push(`<p>${renderInline(escapeHtml(text))}</p>`);
      paragraph = [];
    };

    for (const rawLine of lines) {
      const line = rawLine.replace(/\s+$/, "");
      if (line.trim() === "") {
        flushParagraph();
        continue;
      }
      const heading = line.match(/^(#{1,6})\s+(.*)$/);
      if (heading && heading[1] && heading[2] !== undefined) {
        flushParagraph();
        const level = heading[1].length;
        out.push(`<h${level}>${renderInline(escapeHtml(heading[2]))}</h${level}>`);
        continue;
      }
      paragraph.push(line.trim());
    }
    flushParagraph();

    return { html: out.join("\n") };
  } catch {
    return { html: `<p>${escapeHtml(normalizeNewlines(md))}</p>` };
  }
}
