/**
 * Loads template files from the user's template directory, falling back to the built-in
 * default for any file that isn't there.
 */
import { readFileSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { TemplateBundle } from '../core/renderer.js';

const HERE = dirname(fileURLToPath(import.meta.url));

/**
 * The built-in templates are in `lib/templates/default` once built, or in
 * `src/templates/default` when running from source under tsx or vitest.
 */
function builtinDir(): string {
  const candidates = [
    join(HERE, '..', 'templates', 'default'),
    join(HERE, '..', '..', 'src', 'templates', 'default'),
  ];
  for (const dir of candidates) {
    if (existsSync(join(dir, 'layout.html'))) return dir;
  }
  // If neither exists, readFileSync will report the missing file.
  return candidates[0]!;
}

/** Read `fileName` from `customDir` if it is there, otherwise from `defaults`. */
function resolveFile(fileName: string, customDir: string | undefined, defaults: string): string {
  if (customDir) {
    const custom = join(customDir, fileName);
    if (existsSync(custom)) return readFileSync(custom, 'utf8');
  }
  return readFileSync(join(defaults, fileName), 'utf8');
}

/**
 * Load a {@link TemplateBundle}. Each file falls back on its own, so a custom directory can
 * override just `theme.css`, for example.
 */
export function loadTemplate(customDir?: string): TemplateBundle {
  const defaults = builtinDir();
  return {
    layout: resolveFile('layout.html', customDir, defaults),
    css: resolveFile('theme.css', customDir, defaults),
    clientJs: resolveFile('client.js', customDir, defaults),
  };
}

export function builtinTemplateDir(): string {
  return builtinDir();
}
