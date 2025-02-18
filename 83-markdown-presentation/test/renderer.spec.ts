import { describe, it, expect } from 'vitest';
import { renderMarkdown, renderSlideHtml, renderDeckHtml, EMPTY_DECK_HTML } from '../src/core/renderer.js';
import type { TemplateBundle } from '../src/core/renderer.js';
import { buildDeck } from '../src/core/slide-model.js';
import type { RawSlide } from '../src/core/slide-model.js';

function raw(...chunks: string[]): RawSlide[] {
  return chunks.map((markdown, index) => ({ index, markdown, startLine: index + 1 }));
}

const TEMPLATE: TemplateBundle = {
  layout:
    '<!doctype html><title>{{TITLE}}</title>{{STYLE}}{{HEAD_EXTRA}}' +
    '<main data-count="{{COUNT}}">{{SLIDES}}</main>{{SCRIPT}}',
  css: '.slide{color:red}',
  clientJs: 'console.log("client");',
};

describe('renderMarkdown — per-element goldens', () => {
  it('renders headings', () => {
    expect(renderMarkdown('# H1\n## H2')).toMatchInlineSnapshot(`"<h1>H1</h1>
<h2>H2</h2>"`);
  });

  it('renders unordered lists', () => {
    expect(renderMarkdown('- a\n- b')).toMatchInlineSnapshot(`"<ul>
<li>a</li>
<li>b</li>
</ul>"`);
  });

  it('renders fenced code with a language class', () => {
    const html = renderMarkdown('```ts\nconst x = 1;\n```');
    expect(html).toContain('<pre><code class="language-ts">');
    expect(html).toContain('const x = 1;');
  });

  it('renders inline formatting and links', () => {
    expect(renderMarkdown('**bold** _em_ [l](https://x.com)')).toContain('<strong>bold</strong>');
    expect(renderMarkdown('[l](https://x.com)')).toContain('<a href="https://x.com">l</a>');
  });

  it('renders tables', () => {
    const html = renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |');
    expect(html).toContain('<table>');
    expect(html).toContain('<td>1</td>');
  });

  it('renders blockquotes', () => {
    expect(renderMarkdown('> quote')).toContain('<blockquote>');
  });

  it('rewrites relative image sources beneath the asset prefix', () => {
    const html = renderMarkdown('![alt](images/pic.png)', { assetPrefix: '/assets/' });
    expect(html).toContain('src="/assets/images/pic.png"');
  });

  it('leaves absolute and root-relative image sources untouched', () => {
    expect(renderMarkdown('![](https://x.com/a.png)', { assetPrefix: '/assets/' })).toContain(
      'src="https://x.com/a.png"',
    );
    expect(renderMarkdown('![](/already/a.png)', { assetPrefix: '/assets/' })).toContain(
      'src="/already/a.png"',
    );
  });
});

describe('renderSlideHtml', () => {
  it('wraps a slide in a section with id, index and optional layout', () => {
    const deck = buildDeck({}, raw('<!-- layout: cover -->\n# Hi'));
    const html = renderSlideHtml(deck.slides[0]!);
    expect(html).toContain('<section class="slide" id="slide-1" data-index="0" data-layout="cover">');
    expect(html).toContain('<h1>Hi</h1>');
  });
});

describe('renderDeckHtml — template injection', () => {
  it('injects every slide in order with stable ids and the right count', () => {
    const deck = buildDeck({ title: 'My Talk' }, raw('# One', '# Two', '# Three'));
    const html = renderDeckHtml(deck, TEMPLATE);

    expect(html).toContain('<title>My Talk</title>');
    expect(html).toContain('data-count="3"');
    expect(html).toContain('<style>\n.slide{color:red}\n</style>');
    expect(html).toContain('console.log("client");');

    const o = html.indexOf('id="slide-1"');
    const t = html.indexOf('id="slide-2"');
    const th = html.indexOf('id="slide-3"');
    expect(o).toBeGreaterThan(-1);
    expect(o).toBeLessThan(t);
    expect(t).toBeLessThan(th);
  });

  it('renders a friendly placeholder for an empty deck', () => {
    const deck = buildDeck({}, []);
    const html = renderDeckHtml(deck, TEMPLATE);
    expect(deck.count).toBe(0);
    expect(html).toContain(EMPTY_DECK_HTML);
    expect(html).toContain('data-count="0"');
  });

  it('injects head extra (e.g. live-reload snippet)', () => {
    const deck = buildDeck({}, raw('# A'));
    const html = renderDeckHtml(deck, TEMPLATE, { headExtra: '<!--LR-->' });
    expect(html).toContain('<!--LR-->');
  });

  it('matches the full-page golden snapshot', () => {
    const deck = buildDeck({ title: 'Golden' }, raw('# Slide one\n\n- a\n- b', '## Slide two'));
    expect(renderDeckHtml(deck, TEMPLATE)).toMatchSnapshot();
  });
});
