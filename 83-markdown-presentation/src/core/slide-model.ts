/** The deck data model. */
import { renderMarkdown } from './renderer.js';

/** Deck settings, usually from front matter. */
export interface DeckConfig {
  title?: string;
  /** Built-in theme name, e.g. "default". */
  theme?: string;
  templateDir?: string;
}

/** One slide's Markdown as cut out by the splitter. */
export interface RawSlide {
  index: number;
  markdown: string;
  /** 1-based line in the source file where the slide's content starts. */
  startLine: number;
}

/** A rendered slide. */
export interface Slide {
  /** e.g. "slide-1" */
  id: string;
  index: number;
  html: string;
  markdown: string;
  /** From a `<!-- layout: x -->` comment. */
  layout?: string;
  /** From a `<!-- notes: ... -->` comment. */
  notes?: string;
}

export interface Deck {
  config: DeckConfig;
  slides: Slide[];
  count: number;
}

export interface BuildOptions {
  /** Prepended to relative image paths, e.g. "/assets/". */
  assetPrefix?: string;
}

const LAYOUT_DIRECTIVE = /<!--\s*layout:\s*([^\s>][^>]*?)\s*-->/i;
const NOTES_DIRECTIVE = /<!--\s*notes:\s*([\s\S]*?)-->/i;

/**
 * Render the raw slides into a {@link Deck}. Blank slides are dropped, except that a file
 * containing only whitespace still gets its one slide, so it can be told apart from an
 * empty file.
 */
export function buildDeck(
  config: DeckConfig,
  raw: RawSlide[],
  options: BuildOptions = {},
): Deck {
  const nonEmpty = raw.filter((s) => s.markdown.trim() !== '');
  const kept = nonEmpty.length === 0 && raw.length === 1 ? raw : nonEmpty;

  const slides: Slide[] = kept.map((rawSlide, i) => {
    const layoutMatch = LAYOUT_DIRECTIVE.exec(rawSlide.markdown);
    const notesMatch = NOTES_DIRECTIVE.exec(rawSlide.markdown);
    const slide: Slide = {
      id: `slide-${i + 1}`,
      index: i,
      html: renderMarkdown(rawSlide.markdown, options),
      markdown: rawSlide.markdown,
    };
    if (layoutMatch && layoutMatch[1]) slide.layout = layoutMatch[1].trim();
    if (notesMatch && notesMatch[1]) slide.notes = notesMatch[1].trim();
    return slide;
  });

  return { config, slides, count: slides.length };
}
