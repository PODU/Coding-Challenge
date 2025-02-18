// Copy the built-in templates into the compiled output so the published package ships them.
import { cpSync, existsSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const src = join(root, 'src', 'templates');
const dest = join(root, 'lib', 'templates');

if (existsSync(src)) {
  mkdirSync(dirname(dest), { recursive: true });
  cpSync(src, dest, { recursive: true });
  console.log(`Copied templates → ${dest}`);
} else {
  console.warn('No src/templates directory to copy.');
}
