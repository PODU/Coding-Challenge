/// <reference types="vitest/config" />
import { defineConfig } from "vitest/config";

export default defineConfig({
  base: "./",
  build: {
    target: "es2020",
    outDir: "dist",
    sourcemap: true,
  },
  test: {
    environment: "node",
    include: ["test/**/*.test.ts"],
    environmentMatchGlobs: [["test/**/dom*.test.ts", "jsdom"]],
  },
});
