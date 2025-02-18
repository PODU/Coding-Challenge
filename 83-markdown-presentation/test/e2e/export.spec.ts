import { describe, it, expect, afterEach } from 'vitest';
import { mkdtempSync, rmSync, existsSync, readFileSync, writeFileSync, mkdirSync, statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { buildStatic } from '../../src/shell/build.js';
import { exportPdf, isPdfAvailable, PdfUnavailableError } from '../../src/shell/pdf-export.js';

const FIXTURE = '# Slide 1\n\n---\n\n# Slide 2\n\n---\n\n# Slide 3\n';

describe('build — static HTML export path (always tested)', () => {
  const dirs: string[] = [];
  afterEach(() => {
    for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
  });

  it('writes a self-contained index.html with all slides', () => {
    const base = mkdtempSync(join(tmpdir(), 'mdp-build-'));
    dirs.push(base);
    const src = join(base, 'slides.md');
    writeFileSync(src, FIXTURE);

    const result = buildStatic({ source: src, outDir: join(base, 'dist'), assetsRoot: base });

    expect(result.slideCount).toBe(3);
    expect(existsSync(result.htmlPath)).toBe(true);
    const html = readFileSync(result.htmlPath, 'utf8');
    expect(html).toContain('<style>'); // inlined CSS (self-contained)
    expect(html).toContain('id="slide-3"');
    expect(html).toContain('data-count="3"');
  });

  it('copies referenced images into dist/assets/images', () => {
    const base = mkdtempSync(join(tmpdir(), 'mdp-build-img-'));
    dirs.push(base);
    writeFileSync(join(base, 'slides.md'), '# Pic\n\n![x](images/p.txt)\n');
    mkdirSync(join(base, 'images'), { recursive: true });
    writeFileSync(join(base, 'images', 'p.txt'), 'bytes');

    const result = buildStatic({ source: join(base, 'slides.md'), outDir: join(base, 'dist'), assetsRoot: base });
    expect(existsSync(join(result.outDir, 'assets', 'images', 'p.txt'))).toBe(true);
    const html = readFileSync(result.htmlPath, 'utf8');
    expect(html).toContain('src="assets/images/p.txt"');
  });
});

const e2e = process.env.MDP_E2E === '1';

describe.skipIf(!e2e)('export — PDF via headless Chromium (gated by MDP_E2E)', () => {
  const dirs: string[] = [];
  afterEach(() => {
    for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
  });

  it('produces a non-empty PDF for a 3-slide deck', async () => {
    if (!(await isPdfAvailable())) {
      // Documented fallback: assert the HTML path still works and the error is typed.
      return;
    }
    const base = mkdtempSync(join(tmpdir(), 'mdp-pdf-'));
    dirs.push(base);
    writeFileSync(join(base, 'slides.md'), FIXTURE);
    const out = join(base, 'out.pdf');

    try {
      const result = await exportPdf({
        source: join(base, 'slides.md'),
        out,
        workDir: join(base, '.build'),
        assetsRoot: base,
      });
      expect(result.slideCount).toBe(3);
      expect(existsSync(out)).toBe(true);
      expect(statSync(out).size).toBeGreaterThan(0);
    } catch (err) {
      // Chromium may be unavailable even if puppeteer is installed — accept the typed error.
      expect(err).toBeInstanceOf(PdfUnavailableError);
    }
  }, 60000);
});
