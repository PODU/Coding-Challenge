/**
 * Static build: a standalone `index.html` plus copied images. PDF export prints this page.
 */
import { mkdirSync, writeFileSync, existsSync, cpSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { renderDeckHtml } from '../core/renderer.js';
import { loadDeck } from './pipeline.js';
import { loadTemplate } from './template-loader.js';

export interface BuildStaticOptions {
  source: string;
  outDir: string;
  templateDir?: string;
  /** Directory containing `images/`. Defaults to the cwd. */
  assetsRoot?: string;
}

export interface BuildResult {
  outDir: string;
  htmlPath: string;
  slideCount: number;
}

/** Render `source` into a standalone deck under `outDir`. */
export function buildStatic(options: BuildStaticOptions): BuildResult {
  const outDir = resolve(options.outDir);
  const assetsRoot = options.assetsRoot ?? process.cwd();

  // A relative prefix keeps the output working from file:// or any URL path.
  const deck = loadDeck(options.source, { assetPrefix: 'assets/' });
  const template = loadTemplate(options.templateDir);
  const html = renderDeckHtml(deck, template);

  mkdirSync(outDir, { recursive: true });
  const htmlPath = join(outDir, 'index.html');
  writeFileSync(htmlPath, html, 'utf8');

  const imagesSrc = join(assetsRoot, 'images');
  if (existsSync(imagesSrc)) {
    cpSync(imagesSrc, join(outDir, 'assets', 'images'), { recursive: true });
  }

  return { outDir, htmlPath, slideCount: deck.count };
}
