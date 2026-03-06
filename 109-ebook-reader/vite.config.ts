import { defineConfig } from 'vite';

// The reader UI is a plain browser app. The pure parser/reader modules it
// imports run unchanged in the browser (fflate + fast-xml-parser are pure JS).
export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist-ui',
    emptyOutDir: true,
  },
});
