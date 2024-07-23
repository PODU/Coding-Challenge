// Bundles the vanilla TypeScript SPA with esbuild (prebuilt binary — no native
// compile step), then copies the static HTML shell into web/dist.
import { build } from 'esbuild';
import { mkdirSync, copyFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const outdir = resolve(root, 'web', 'dist');
mkdirSync(outdir, { recursive: true });

await build({
  entryPoints: [resolve(root, 'web', 'src', 'main.ts')],
  bundle: true,
  format: 'esm',
  target: 'es2020',
  sourcemap: true,
  minify: true,
  outfile: resolve(outdir, 'bundle.js'),
  logLevel: 'info',
});

copyFileSync(resolve(root, 'web', 'index.html'), resolve(outdir, 'index.html'));
copyFileSync(resolve(root, 'web', 'styles.css'), resolve(outdir, 'styles.css'));

console.log('web build complete ->', outdir);
