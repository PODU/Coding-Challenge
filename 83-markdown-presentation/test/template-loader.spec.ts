import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { loadTemplate } from '../src/shell/template-loader.js';

describe('template loader — custom dir with built-in fallback (Step 4)', () => {
  let dir: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), 'mdp-tmpl-'));
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it('uses the built-in defaults when the custom dir is empty', () => {
    const bundle = loadTemplate(dir);
    expect(bundle.layout).toContain('{{SLIDES}}');
    expect(bundle.css.length).toBeGreaterThan(0);
    expect(bundle.clientJs).toContain('clampIndex');
  });

  it('overrides only the provided file and falls back for the rest', () => {
    writeFileSync(join(dir, 'theme.css'), '/* custom */ body{background:hotpink}');
    const bundle = loadTemplate(dir);
    expect(bundle.css).toContain('hotpink'); // custom
    expect(bundle.layout).toContain('{{SLIDES}}'); // default fallback
    expect(bundle.clientJs).toContain('clampIndex'); // default fallback
  });

  it('uses a fully custom layout when provided', () => {
    writeFileSync(join(dir, 'layout.html'), '<html>{{SLIDES}}::{{COUNT}}</html>');
    const bundle = loadTemplate(dir);
    expect(bundle.layout).toBe('<html>{{SLIDES}}::{{COUNT}}</html>');
  });

  it('falls back to defaults when given a non-existent custom dir', () => {
    const bundle = loadTemplate(join(dir, 'does-not-exist'));
    expect(bundle.layout).toContain('{{SLIDES}}');
  });

  it('with no custom dir returns the built-in bundle', () => {
    const bundle = loadTemplate();
    expect(bundle.layout).toContain('<!DOCTYPE html>');
  });
});
