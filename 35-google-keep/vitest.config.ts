import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    environment: 'node',
    globals: false,
    coverage: {
      reporter: ['text', 'html'],
      include: ['src/**/*.ts'],
    },
  },
});
