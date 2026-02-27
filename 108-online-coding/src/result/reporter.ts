import type { ErrorInfo, Language, RawExecution, RunResult } from '../types';
import { parseStderr } from './traceback';

/** Turn a raw sandbox execution into a RunResult with a status and ErrorInfo. */
export function buildRunResult(
  language: Language,
  raw: RawExecution,
  timeoutMs: number,
): RunResult {
  const base = {
    exitCode: raw.exitCode,
    signal: raw.signal,
    stdout: raw.stdout,
    stderr: raw.stderr,
    timeMs: raw.timeMs,
    truncated: raw.truncated,
  };

  if (raw.internalError) {
    const error: ErrorInfo = {
      type: 'internal',
      message: raw.internalMessage ?? 'Internal execution error',
      line: null,
      traceback: raw.internalMessage ?? null,
    };
    return { ...base, status: 'internal_error', error };
  }

  if (raw.timedOut) {
    const error: ErrorInfo = {
      type: 'timeout',
      message: `Execution exceeded the ${(timeoutMs / 1000).toFixed(1)}s time limit`,
      line: null,
      traceback: raw.stderr || null,
    };
    return { ...base, status: 'timeout', error };
  }

  if (raw.outputLimited) {
    const error: ErrorInfo = {
      type: 'output_limit',
      message: 'Output limit exceeded — program produced too much output',
      line: null,
      traceback: null,
    };
    return { ...base, status: 'output_limit', error };
  }

  // Running out of memory shows up as an ordinary runtime error (Python
  // MemoryError, Node heap OOM), so look for it in stderr.
  if (isMemoryError(raw.stderr)) {
    const error: ErrorInfo = {
      type: 'memory_limit',
      message: 'Memory limit exceeded',
      line: null,
      traceback: raw.stderr || null,
    };
    return { ...base, status: 'memory_limit', error };
  }

  if (raw.exitCode === 0 && raw.signal === null) {
    return { ...base, status: 'ok', error: null };
  }

  // Non-zero exit (or killed by signal) -> error. Parse stderr when present.
  if (raw.stderr.trim() !== '') {
    const parsed = parseStderr(language, raw.stderr);
    const error: ErrorInfo = {
      type: parsed.type,
      message: parsed.message,
      line: parsed.line,
      traceback: raw.stderr,
    };
    return { ...base, status: 'error', error };
  }

  const error: ErrorInfo = {
    type: 'runtime',
    message:
      raw.signal !== null
        ? `Process terminated by signal ${raw.signal}`
        : `Process exited with code ${raw.exitCode ?? 'unknown'}`,
    line: null,
    traceback: null,
  };
  return { ...base, status: 'error', error };
}

function isMemoryError(stderr: string): boolean {
  return (
    /\bMemoryError\b/.test(stderr) ||
    /JavaScript heap out of memory/.test(stderr) ||
    /Cannot allocate memory/.test(stderr)
  );
}
