import { spawn } from 'node:child_process';
import treeKill from 'tree-kill';
import type { RawExecution } from '../types';

export interface SpawnSpec {
  /** Executable to run (interpreter / node). */
  command: string;
  /** Argument vector. */
  args: string[];
  /** Working directory (a fresh per-run temp dir). */
  cwd: string;
  /** Scrubbed environment for the child. */
  env: NodeJS.ProcessEnv;
  /** Data fed to the child's stdin. */
  stdin: string;
  /** Wall-clock deadline in ms. */
  timeoutMs: number;
  /** Combined stdout+stderr byte ceiling. */
  outputLimitBytes: number;
  /** Grace period after SIGKILL before resolving anyway. */
  killGraceMs: number;
}

/**
 * Production can swap ProcessSandbox for a container-based sandbox
 * without touching callers.
 */
export interface Sandbox {
  run(spec: SpawnSpec): Promise<RawExecution>;
}

/**
 * Kill an entire process tree (defeats fork bombs / child-spawned processes).
 * Cross-platform: tree-kill shells to `taskkill /T /F` on Windows and walks
 * /proc on POSIX. `detached` puts the child in its own group on POSIX.
 */
function killTree(pid: number, signal: NodeJS.Signals = 'SIGKILL'): void {
  treeKill(pid, signal, () => {
    /* best-effort; errors mean the process already exited */
  });
}

/**
 * Process-based sandbox. It enforces:
 *  - hard wall-clock deadline -> SIGKILL the whole process group
 *  - bounded stdout/stderr capture -> kill + flag on overflow
 *  - detached process group so one kill reaps child processes too
 *  - scrubbed env + caller-supplied isolated-mode interpreter flags
 *
 * NOTE: this shares the host kernel. Production MUST wrap each run in a
 * container/gVisor/microVM. See README "Security".
 */
export class ProcessSandbox implements Sandbox {
  run(spec: SpawnSpec): Promise<RawExecution> {
    return new Promise<RawExecution>((resolve) => {
      const start = process.hrtime.bigint();

      let settled = false;
      let timedOut = false;
      let outputLimited = false;
      let truncated = false;

      const stdoutChunks: Buffer[] = [];
      const stderrChunks: Buffer[] = [];
      let capturedBytes = 0;

      let child;
      try {
        child = spawn(spec.command, spec.args, {
          cwd: spec.cwd,
          env: spec.env,
          // detached: own process group on POSIX so kill -pid reaps the tree.
          detached: process.platform !== 'win32',
          stdio: ['pipe', 'pipe', 'pipe'],
          windowsHide: true,
        });
      } catch (err) {
        resolve({
          exitCode: null,
          signal: null,
          stdout: '',
          stderr: '',
          timeMs: 0,
          truncated: false,
          timedOut: false,
          outputLimited: false,
          internalError: true,
          internalMessage: err instanceof Error ? err.message : String(err),
        });
        return;
      }

      const pid = child.pid;

      const elapsedMs = (): number =>
        Number(process.hrtime.bigint() - start) / 1_000_000;

      const finish = (over: Partial<RawExecution>, code: number | null, signal: string | null): void => {
        if (settled) return;
        settled = true;
        clearTimeout(deadline);
        resolve({
          exitCode: code,
          signal,
          stdout: Buffer.concat(stdoutChunks).toString('utf8'),
          stderr: Buffer.concat(stderrChunks).toString('utf8'),
          timeMs: Math.round(elapsedMs()),
          truncated,
          timedOut,
          outputLimited,
          internalError: false,
          ...over,
        });
      };

      const deadline = setTimeout(() => {
        timedOut = true;
        if (pid !== undefined) killTree(pid);
        // If the kill is ignored, resolve after a grace period anyway so the
        // request (and the test suite) never hangs.
        setTimeout(() => finish({}, null, 'SIGKILL'), spec.killGraceMs).unref();
      }, spec.timeoutMs);
      deadline.unref();

      const onData = (chunks: Buffer[]) => (buf: Buffer): void => {
        if (outputLimited) return;
        const remaining = spec.outputLimitBytes - capturedBytes;
        if (buf.length >= remaining) {
          chunks.push(buf.subarray(0, Math.max(0, remaining)));
          capturedBytes = spec.outputLimitBytes;
          outputLimited = true;
          truncated = true;
          if (pid !== undefined) killTree(pid);
          setTimeout(() => finish({}, null, 'SIGKILL'), spec.killGraceMs).unref();
          return;
        }
        chunks.push(buf);
        capturedBytes += buf.length;
      };

      child.stdout.on('data', onData(stdoutChunks));
      child.stderr.on('data', onData(stderrChunks));

      child.on('error', (err) => {
        finish(
          { internalError: true, internalMessage: err.message },
          null,
          null,
        );
      });

      child.on('close', (code, signal) => {
        finish({}, code, signal ?? null);
      });

      child.stdin.on('error', () => {
        /* child may exit before we finish writing; ignore EPIPE */
      });
      child.stdin.end(spec.stdin);
    });
  }
}
