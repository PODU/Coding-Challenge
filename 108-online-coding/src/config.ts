/**
 * Central runtime configuration. Every limit is overridable via env so tests
 * can use short, non-hanging values.
 */

function intEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw.trim() === '') return fallback;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

export const config = {
  /** HTTP port. */
  port: intEnv('PORT', 3000),

  /** Default wall-clock deadline for a single run. */
  defaultTimeoutMs: intEnv('RUN_TIMEOUT_MS', 5_000),

  /** Hard ceiling for any per-case/per-run timeout a caller can request. */
  maxTimeoutMs: intEnv('RUN_MAX_TIMEOUT_MS', 15_000),

  /** Grace period after SIGKILL before we give up waiting on the process. */
  killGraceMs: intEnv('RUN_KILL_GRACE_MS', 2_000),

  /** Combined stdout+stderr byte ceiling before we kill + flag truncation. */
  outputLimitBytes: intEnv('RUN_OUTPUT_LIMIT_BYTES', 256 * 1024),

  /** Max accepted source length in bytes (mapped to 413). */
  maxSourceBytes: intEnv('RUN_MAX_SOURCE_BYTES', 64 * 1024),

  /** Max accepted stdin length in bytes. */
  maxStdinBytes: intEnv('RUN_MAX_STDIN_BYTES', 64 * 1024),

  /** Max simultaneous untrusted processes (bounded worker pool). */
  maxConcurrency: intEnv('RUN_MAX_CONCURRENCY', 4),

  /** Max queued runs waiting for a slot before we shed load with 429. */
  maxQueueLength: intEnv('RUN_MAX_QUEUE', 32),

  /** Simple per-IP token bucket: requests allowed per window. */
  rateLimitMax: intEnv('RATE_LIMIT_MAX', 60),
  rateLimitWindowMs: intEnv('RATE_LIMIT_WINDOW_MS', 60_000),

  /** Python interpreter binary. Defaults per-platform; override with PYTHON_BIN. */
  pythonBin:
    process.env.PYTHON_BIN ?? (process.platform === 'win32' ? 'python' : 'python3'),

  /** Node binary used to run untrusted JS. */
  nodeBin: process.env.NODE_BIN ?? process.execPath,

  version: '1.0.0',
} as const;

export type AppConfig = typeof config;
