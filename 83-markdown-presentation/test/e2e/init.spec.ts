import { describe, it, expect, afterEach } from 'vitest';
import { mkdtempSync, rmSync, existsSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { scaffold } from '../../src/shell/scaffold.js';

describe('init / scaffold (Step 1)', () => {
  const dirs: string[] = [];

  afterEach(() => {
    for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
  });

  it('creates the opinionated project layout', () => {
    const base = mkdtempSync(join(tmpdir(), 'mdp-init-'));
    dirs.push(base);
    const dir = join(base, 'deck');

    const result = scaffold({ dir, git: false });

    expect(existsSync(join(dir, 'slides', 'slides.md'))).toBe(true);
    expect(existsSync(join(dir, 'templates'))).toBe(true);
    expect(existsSync(join(dir, 'images'))).toBe(true);
    expect(existsSync(join(dir, 'README.md'))).toBe(true);
    expect(existsSync(join(dir, 'mdpresent.config.json'))).toBe(true);

    const config = JSON.parse(readFileSync(join(dir, 'mdpresent.config.json'), 'utf8'));
    expect(config.theme).toBe('default');
    expect(result.created.length).toBeGreaterThan(0);
  });

  it('is idempotent — does not overwrite existing files', () => {
    const base = mkdtempSync(join(tmpdir(), 'mdp-init2-'));
    dirs.push(base);
    const dir = join(base, 'deck');

    scaffold({ dir, git: false });
    const second = scaffold({ dir, git: false });
    expect(second.created).toHaveLength(0);
  });
});
