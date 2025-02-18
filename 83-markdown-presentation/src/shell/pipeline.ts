/** Turns a Markdown file or directory on disk into a {@link Deck}. */
import { readFileSync, statSync, readdirSync } from 'node:fs';
import { join, extname } from 'node:path';
import { parseFrontMatter } from '../core/frontmatter.js';
import { splitSlides } from '../core/splitter.js';
import { buildDeck } from '../core/slide-model.js';
import type { Deck, BuildOptions } from '../core/slide-model.js';

/** Read a `.md` file, or all `.md` files in a directory joined as separate slides. */
export function readSource(path: string): string {
  const stat = statSync(path);
  if (stat.isDirectory()) {
    const files = readdirSync(path)
      .filter((f) => extname(f).toLowerCase() === '.md')
      .sort();
    return files.map((f) => readFileSync(join(path, f), 'utf8')).join('\n\n---\n\n');
  }
  return readFileSync(path, 'utf8');
}

/** Build a {@link Deck} from Markdown source. */
export function buildDeckFromSource(source: string, options: BuildOptions = {}): Deck {
  const { data, body, bodyStartLine } = parseFrontMatter(source);
  const raw = splitSlides(body, bodyStartLine - 1);
  return buildDeck(data, raw, options);
}

/** Read a path from disk and build its {@link Deck}. */
export function loadDeck(path: string, options: BuildOptions = {}): Deck {
  return buildDeckFromSource(readSource(path), options);
}
