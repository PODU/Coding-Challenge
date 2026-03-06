import { describe, it, expect } from 'vitest';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { parseEpub } from '../src/parser/index.js';
import { fixture } from './helpers.js';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const cli = join(root, 'src', 'cli.ts');
const fx = (n: string) => join(here, 'fixtures', n);

function runCli(args: string[]): string {
  // Run via the tsx ESM loader without a shell so paths with spaces are safe.
  return execFileSync(process.execPath, ['--import', 'tsx', cli, ...args], {
    cwd: root,
    encoding: 'utf8',
  });
}

describe('16. parseEpub end-to-end', () => {
  it('produces a Book whose stages are wired together', async () => {
    const book = await parseEpub(fixture('minimal-v3.epub'));
    expect({
      version: book.version,
      title: book.metadata.title,
      authors: book.metadata.authors,
      spineLength: book.spine.length,
      tocTop: book.toc.map((t) => t.label),
      cover: book.coverHref,
      firstChapterHasMarker: book.getChapter(0).html.includes('MARKER-CHAPTER-ONE'),
    }).toEqual({
      version: '3.0',
      title: 'The Test Book',
      authors: ['Ada Lovelace'],
      spineLength: 4,
      tocTop: ['Chapter One', 'Chapter Two', 'Chapter Three'],
      cover: 'OEBPS/images/cover.png',
      firstChapterHasMarker: true,
    });
  });
});

describe('17. CLI output', () => {
  it('info prints title/author/version', () => {
    const out = runCli(['info', fx('minimal-v3.epub')]);
    expect(out).toContain('Title:      The Test Book');
    expect(out).toContain('Author:     Ada Lovelace');
    expect(out).toContain('Version:    3.0');
  });

  it('toc prints the tree', () => {
    const out = runCli(['toc', fx('minimal-v2.epub')]);
    expect(out).toContain('- Chapter One');
    expect(out).toContain('  - Part A');
  });

  it('spine prints reading order with non-linear flag', () => {
    const out = runCli(['spine', fx('minimal-v3.epub')]);
    expect(out).toContain('OEBPS/text/chap1.xhtml');
    expect(out).toContain('[non-linear]');
  });

  it('text dumps plain-text chapter content', () => {
    const out = runCli(['text', fx('minimal-v3.epub'), '0']);
    expect(out).toContain('MARKER-CHAPTER-ONE');
    expect(out).not.toContain('<p>');
  });
}, 60000);
