import { defineConfig } from 'vite';
import { resolve } from 'node:path';

// Multi-page browser client. `npm run dev:web` serves with HMR and proxies
// /api and /v to the Express backend on :3000. `npm run build:web` emits to web/dist,
// which the Express server then serves statically.
export default defineConfig({
  root: resolve(__dirname, 'web'),
  build: {
    outDir: resolve(__dirname, 'web/dist'),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        home: resolve(__dirname, 'web/index.html'),
        record: resolve(__dirname, 'web/record.html'),
        view: resolve(__dirname, 'web/v.html'),
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:3000',
    },
  },
});
