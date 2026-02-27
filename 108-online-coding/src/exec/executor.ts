import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { config } from '../config';
import type { RunnerRegistry } from '../runners/registry';
import { maybeWrapWithUlimit } from '../sandbox/ulimit';
import { ProcessSandbox, type Sandbox } from '../sandbox/sandbox';
import { buildRunResult } from '../result/reporter';
import type { Language, RunResult } from '../types';
import { WorkerPool } from './queue';

export interface RunRequest {
  language: Language;
  source: string;
  stdin?: string;
  timeoutMs?: number;
}

/**
 * Runs one submission: write it to a fresh temp dir, run it in the sandbox,
 * build the result, and delete the temp dir. A shared WorkerPool limits how
 * many run at once.
 */
export class Executor {
  private readonly pool: WorkerPool;

  constructor(
    private readonly registry: RunnerRegistry,
    private readonly sandbox: Sandbox = new ProcessSandbox(),
    pool?: WorkerPool,
  ) {
    this.pool = pool ?? new WorkerPool(config.maxConcurrency, config.maxQueueLength);
  }

  get poolStats(): { active: number; queued: number } {
    return { active: this.pool.activeCount, queued: this.pool.queuedCount };
  }

  /** Enqueue a run; rejects with QueueFullError when saturated. */
  run(req: RunRequest): Promise<RunResult> {
    return this.pool.submit(() => this.executeOnce(req));
  }

  private async executeOnce(req: RunRequest): Promise<RunResult> {
    const runner = this.registry.get(req.language);
    const timeoutMs = clampTimeout(req.timeoutMs);

    let workdir: string | null = null;
    try {
      workdir = await mkdtemp(join(tmpdir(), 'ocp-run-'));
      const programPath = join(workdir, runner.fileName);
      await writeFile(programPath, req.source, 'utf8');

      const built = runner.buildCommand(programPath);
      const wrapped = maybeWrapWithUlimit(built);

      const raw = await this.sandbox.run({
        command: wrapped.command,
        args: wrapped.args,
        cwd: workdir,
        env: runner.scrubbedEnv(),
        stdin: req.stdin ?? '',
        timeoutMs,
        outputLimitBytes: config.outputLimitBytes,
        killGraceMs: config.killGraceMs,
      });

      return buildRunResult(req.language, raw, timeoutMs);
    } finally {
      if (workdir) {
        await rm(workdir, { recursive: true, force: true }).catch(() => {
          /* best-effort cleanup */
        });
      }
    }
  }
}

export function clampTimeout(requested?: number): number {
  if (requested === undefined || !Number.isFinite(requested) || requested <= 0) {
    return config.defaultTimeoutMs;
  }
  return Math.min(requested, config.maxTimeoutMs);
}
