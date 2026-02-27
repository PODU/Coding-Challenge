import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    include: ['test/**/*.test.ts'],
    // Spawning real interpreters on Windows can be slow; keep generous timeouts
    // but never long enough to "hang" — limit tests use short run deadlines.
    testTimeout: 30_000,
    hookTimeout: 30_000,
    // Run files sequentially so concurrent subprocess spawns don't contend.
    fileParallelism: false,
  },
});
