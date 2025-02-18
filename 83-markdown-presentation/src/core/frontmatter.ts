/**
 * Front matter parsing. Front matter and slide separators are both `---`, so this has to
 * run before the splitter.
 */
import matter from 'gray-matter';
import type { DeckConfig } from './slide-model.js';

export interface FrontMatterResult {
  data: DeckConfig;
  /** Document body with the front-matter block removed. */
  body: string;
  /** 1-based source line where the body starts. */
  bodyStartLine: number;
}

/** Convert CRLF and CR line endings to LF. */
export function normalizeNewlines(src: string): string {
  return src.replace(/\r\n?/g, '\n');
}

function coerceConfig(data: Record<string, unknown>): DeckConfig {
  const cfg: DeckConfig = {};
  if (typeof data.title === 'string') cfg.title = data.title;
  if (typeof data.theme === 'string') cfg.theme = data.theme;
  if (typeof data.templateDir === 'string') cfg.templateDir = data.templateDir;
  return cfg;
}

export function parseFrontMatter(src: string): FrontMatterResult {
  const normalized = normalizeNewlines(src);

  // Without front matter, gray-matter returns the input unchanged as content.
  const parsed = matter(normalized);
  const body = parsed.content;
  const data = coerceConfig(parsed.data as Record<string, unknown>);

  // Count the lines the front matter took up, so slide line numbers match the file.
  let bodyStartLine = 1;
  if (body !== normalized) {
    const idx = normalized.indexOf(body);
    const consumed = idx >= 0 ? normalized.slice(0, idx) : '';
    bodyStartLine = consumed.split('\n').length;
  }

  return { data, body, bodyStartLine };
}
