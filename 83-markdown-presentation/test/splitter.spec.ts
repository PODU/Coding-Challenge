import { describe, it, expect } from 'vitest';
import { splitSlides } from '../src/core/splitter.js';
import { parseFrontMatter, normalizeNewlines } from '../src/core/frontmatter.js';

/** Convenience: split full source the way the pipeline does (front matter consumed first). */
function split(src: string) {
  const { body, bodyStartLine } = parseFrontMatter(src);
  return splitSlides(body, bodyStartLine - 1);
}

describe('splitSlides', () => {
  it('returns [] for empty input', () => {
    expect(splitSlides('')).toEqual([]);
  });

  it('treats a document with no separators as a single slide', () => {
    const slides = splitSlides('# Hello\n\nWorld');
    expect(slides).toHaveLength(1);
    expect(slides[0]!.markdown).toBe('# Hello\n\nWorld');
    expect(slides[0]!.startLine).toBe(1);
  });

  it('splits n separators into n+1 raw chunks', () => {
    const slides = splitSlides('A\n---\nB\n---\nC');
    expect(slides.map((s) => s.markdown)).toEqual(['A', 'B', 'C']);
    expect(slides).toHaveLength(3);
  });

  it('accepts thematic breaks of more than three hyphens', () => {
    const slides = splitSlides('A\n-----\nB');
    expect(slides).toHaveLength(2);
  });

  it('produces an empty leading chunk for a leading separator', () => {
    const slides = splitSlides('---\nA');
    expect(slides.map((s) => s.markdown)).toEqual(['', 'A']);
  });

  it('produces an empty trailing chunk for a trailing separator', () => {
    const slides = splitSlides('A\n---');
    expect(slides.map((s) => s.markdown)).toEqual(['A', '']);
  });

  it('does NOT split on `---` inside a ``` fenced block', () => {
    const src = ['```', 'a', '---', 'b', '```'].join('\n');
    const slides = splitSlides(src);
    expect(slides).toHaveLength(1);
    expect(slides[0]!.markdown).toContain('---');
  });

  it('does NOT split on `---` inside a ~~~ fenced block', () => {
    const src = ['~~~', '---', '~~~'].join('\n');
    expect(splitSlides(src)).toHaveLength(1);
  });

  it('handles a longer closing fence and ignores shorter inner fences', () => {
    const src = ['````', '```', '---', '```', '````', '---', 'next'].join('\n');
    const slides = splitSlides(src);
    expect(slides).toHaveLength(2);
    expect(slides[0]!.markdown).toContain('---');
    expect(slides[1]!.markdown).toBe('next');
  });

  it('normalizes CRLF before splitting', () => {
    const src = normalizeNewlines('A\r\n---\r\nB');
    const slides = splitSlides(src);
    expect(slides.map((s) => s.markdown)).toEqual(['A', 'B']);
  });

  it('reports 1-based startLine for each chunk', () => {
    const slides = splitSlides('A\nA2\n---\nB\n---\nC');
    expect(slides.map((s) => s.startLine)).toEqual([1, 4, 6]);
  });

  it('keeps front matter out of the slides and offsets startLine past it', () => {
    const src = ['---', 'title: Demo', 'theme: default', '---', '', '# One', '---', '# Two'].join('\n');
    const { data } = parseFrontMatter(src);
    expect(data.title).toBe('Demo');
    expect(data.theme).toBe('default');

    const slides = split(src);
    expect(slides.map((s) => s.markdown.trim())).toEqual(['# One', '# Two']);
    // Body begins after the 4-line front matter; "# One" is on source line 6.
    expect(slides[0]!.startLine).toBe(6);
    expect(slides[1]!.startLine).toBe(8);
  });

  it('does not treat the front-matter `---` pair as separators', () => {
    const src = ['---', 'title: T', '---', 'only one slide'].join('\n');
    const slides = split(src);
    expect(slides).toHaveLength(1);
    expect(slides[0]!.markdown.trim()).toBe('only one slide');
  });
});
