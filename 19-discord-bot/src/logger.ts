/**
 * Minimal logger that writes one JSON object per line, which journald, Docker
 * and pm2 can all pick up.
 */

export type LogLevel = "error" | "warn" | "info" | "debug";

const LEVEL_WEIGHT: Record<LogLevel, number> = {
  error: 0,
  warn: 1,
  info: 2,
  debug: 3,
};

export interface Logger {
  level: LogLevel;
  error(msg: string, meta?: Record<string, unknown>): void;
  warn(msg: string, meta?: Record<string, unknown>): void;
  info(msg: string, meta?: Record<string, unknown>): void;
  debug(msg: string, meta?: Record<string, unknown>): void;
}

export function createLogger(level: LogLevel = "info"): Logger {
  const threshold = LEVEL_WEIGHT[level];

  function emit(
    lvl: LogLevel,
    msg: string,
    meta?: Record<string, unknown>,
  ): void {
    if (LEVEL_WEIGHT[lvl] > threshold) return;
    const line = JSON.stringify({
      ts: new Date().toISOString(),
      level: lvl,
      msg,
      ...(meta ?? {}),
    });
    if (lvl === "error" || lvl === "warn") {
      process.stderr.write(line + "\n");
    } else {
      process.stdout.write(line + "\n");
    }
  }

  return {
    level,
    error: (msg, meta) => emit("error", msg, meta),
    warn: (msg, meta) => emit("warn", msg, meta),
    info: (msg, meta) => emit("info", msg, meta),
    debug: (msg, meta) => emit("debug", msg, meta),
  };
}
