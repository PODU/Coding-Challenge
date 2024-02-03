// Copies public/ into dist/ next to the esbuild bundle.
import { cp, mkdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const from = resolve(root, 'public');
const to = resolve(root, 'dist');

await mkdir(to, { recursive: true });
await cp(from, to, { recursive: true });

console.log('Copied public/ -> dist/');
