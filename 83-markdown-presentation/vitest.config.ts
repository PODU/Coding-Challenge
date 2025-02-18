import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['test/**/*.spec.ts'],
    // e2e (server/pdf) runs via `npm run test:e2e`; keep the default unit run fast.
    environment: 'node',
    globals: false,
  },
});
