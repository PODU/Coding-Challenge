/// <reference types="vitest" />
import { defineConfig } from 'vite';

export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    target: 'es2020',
    sourcemap: true,
  },
  test: {
    globals: true,
    environment: 'node',
    include: ['test/**/*.test.ts'],
    environmentMatchGlobs: [['test/ui.test.ts', 'jsdom']],
  },
});
