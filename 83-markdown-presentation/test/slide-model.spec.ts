import { describe, it, expect } from 'vitest';
import { buildDeck } from '../src/core/slide-model.js';
import { buildDeckFromSource } from '../src/shell/pipeline.js';
import type { RawSlide } from '../src/core/slide-model.js';

function raw(...chunks: string[]): RawSlide[] {
  return chunks.map((markdown, index) => ({ index, markdown, startLine: index + 1 }));
}

describe('buildDeck — empty-slide policy', () => {
  it('drops empty leading/trailing slides', () => {
    const deck = buildDeck({}, raw('', '# A', '', '# B', ''));
    expect(deck.count).toBe(2);
    expect(deck.slides.map((s) => s.id)).toEqual(['slide-1', 'slide-2']);
  });

  it('drops all-empty input to a zero-slide deck', () => {
    const deck = buildDeck({}, raw('', '   ', '\n'));
    expect(deck.count).toBe(0);
  });

  it('keeps a single whitespace-only slide (distinct from an empty file)', () => {
    const deck = buildDeck({}, raw('   '));
    expect(deck.count).toBe(1);
  });

  it('assigns stable sequential ids and indexes', () => {
    const deck = buildDeck({}, raw('# A', '# B', '# C'));
    expect(deck.slides.map((s) => s.index)).toEqual([0, 1, 2]);
    expect(deck.slides.map((s) => s.id)).toEqual(['slide-1', 'slide-2', 'slide-3']);
  });

  it('renders markdown into slide html', () => {
    const deck = buildDeck({}, raw('# Title'));
    expect(deck.slides[0]!.html).toContain('<h1>Title</h1>');
  });

  it('extracts a layout directive', () => {
    const deck = buildDeck({}, raw('<!-- layout: two-column -->\n# Hi'));
    expect(deck.slides[0]!.layout).toBe('two-column');
  });
});

describe('buildDeckFromSource — full pipeline', () => {
  it('parses front matter config and counts slides', () => {
    const src = ['---', 'title: Deck', '---', '# One', '---', '# Two'].join('\n');
    const deck = buildDeckFromSource(src);
    expect(deck.config.title).toBe('Deck');
    expect(deck.count).toBe(2);
  });

  it('an only-front-matter document yields zero slides', () => {
    const src = ['---', 'title: Empty', '---', ''].join('\n');
    const deck = buildDeckFromSource(src);
    expect(deck.count).toBe(0);
  });
});
