/**
 * Markdown to HTML. Templates are passed in as strings, so nothing here touches the
 * filesystem.
 */
import MarkdownIt from 'markdown-it';
import type { Deck, Slide, BuildOptions } from './slide-model.js';

export const md: MarkdownIt = new MarkdownIt({
  html: true, // the layout and notes directives are HTML comments
  linkify: true,
  typographer: false, // keep quotes and dashes as written
  breaks: false,
});

/** A loaded template. The CSS and JS are inlined into the page. */
export interface TemplateBundle {
  /** HTML with `{{TOKEN}}` placeholders. */
  layout: string;
  css: string;
  clientJs: string;
}

export interface DeckRenderOptions {
  /** Markup for the `{{HEAD_EXTRA}}` slot, e.g. the live-reload snippet. */
  headExtra?: string;
}

function isRelativeAsset(src: string): boolean {
  return !/^(?:[a-z]+:)?\/\//i.test(src) && !src.startsWith('/') && !src.startsWith('data:');
}

/** Prefix relative image paths with `assetPrefix`. */
function applyAssetPrefix(md_: MarkdownIt, assetPrefix: string): void {
  const defaultImage =
    md_.renderer.rules.image ??
    ((tokens, idx, opts, _env, self) => self.renderToken(tokens, idx, opts));
  md_.renderer.rules.image = (tokens, idx, opts, env, self) => {
    const token = tokens[idx]!;
    const srcIndex = token.attrIndex('src');
    if (srcIndex >= 0) {
      const attr = token.attrs![srcIndex]!;
      const src = attr[1];
      if (isRelativeAsset(src)) {
        attr[1] = assetPrefix.replace(/\/$/, '') + '/' + src.replace(/^\.\//, '');
      }
    }
    return defaultImage(tokens, idx, opts, env, self);
  };
}

/** Render Markdown to an HTML fragment, prefixing relative image paths if asked. */
export function renderMarkdown(source: string, options: BuildOptions = {}): string {
  if (options.assetPrefix) {
    const local = new MarkdownIt({ html: true, linkify: true, typographer: false });
    applyAssetPrefix(local, options.assetPrefix);
    return local.render(source).trimEnd();
  }
  return md.render(source).trimEnd();
}

/** Wrap a slide's HTML in its `<section>`. */
export function renderSlideHtml(slide: Slide): string {
  const layoutAttr = slide.layout ? ` data-layout="${escapeAttr(slide.layout)}"` : '';
  return (
    `<section class="slide" id="${slide.id}" data-index="${slide.index}"${layoutAttr}>\n` +
    `${slide.html}\n` +
    `</section>`
  );
}

function escapeAttr(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/** Shown when a deck has no slides. */
export const EMPTY_DECK_HTML =
  '<section class="slide slide--empty" id="slide-empty" data-index="0">\n' +
  '<h1>No slides yet</h1>\n' +
  '<p>Add Markdown content to get started. Separate slides with a line containing <code>---</code>.</p>\n' +
  '</section>';

/** Fill the template layout with the slides, CSS and JS to make a standalone page. */
export function renderDeckHtml(
  deck: Deck,
  template: TemplateBundle,
  options: DeckRenderOptions = {},
): string {
  const slidesHtml =
    deck.slides.length === 0
      ? EMPTY_DECK_HTML
      : deck.slides.map(renderSlideHtml).join('\n');

  const title = deck.config.title ?? 'Presentation';

  return template.layout
    .replace(/\{\{\s*TITLE\s*\}\}/g, escapeHtml(title))
    .replace(/\{\{\s*COUNT\s*\}\}/g, String(deck.count))
    .replace(/\{\{\s*STYLE\s*\}\}/g, () => `<style>\n${template.css}\n</style>`)
    .replace(/\{\{\s*HEAD_EXTRA\s*\}\}/g, () => options.headExtra ?? '')
    .replace(/\{\{\s*SLIDES\s*\}\}/g, () => slidesHtml)
    .replace(/\{\{\s*SCRIPT\s*\}\}/g, () => `<script>\n${template.clientJs}\n</script>`);
}
