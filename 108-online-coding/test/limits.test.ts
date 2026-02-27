import { describe, it, expect } from 'vitest';
import { tmpdir } from 'node:os';
import { Executor } from '../src/exec/executor';
import { defaultRegistry } from '../src/runners/registry';
import { ProcessSandbox } from '../src/sandbox/sandbox';
import { PythonRunner } from '../src/runners/pythonRunner';
import { config } from '../src/config';

const registry = defaultRegistry();
const executor = new Executor(registry);
const sandbox = new ProcessSandbox();
const pyEnv = new PythonRunner().scrubbedEnv();

describe('limits: timeout', () => {
  it('kills an infinite loop at the deadline and returns timeout', async () => {
    const start = Date.now();
    const r = await executor.run({
      language: 'python',
      source: 'while True:\n    pass\n',
      timeoutMs: 700,
    });
    const elapsed = Date.now() - start;
    expect(r.status).toBe('timeout');
    expect(r.error?.type).toBe('timeout');
    // Completes within deadline + kill grace (no hanging).
    expect(elapsed).toBeLessThan(700 + config.killGraceMs + 5000);
  });
});

describe('limits: output flood', () => {
  it('caps output, kills the process, flags truncation', async () => {
    const raw = await sandbox.run({
      command: config.pythonBin,
      args: ['-I', '-B', '-E', '-c', 'import sys\nwhile True:\n    sys.stdout.write("x" * 1000)\n'],
      cwd: tmpdir(),
      env: pyEnv,
      stdin: '',
      timeoutMs: 5000,
      outputLimitBytes: 4096,
      killGraceMs: 1500,
    });
    expect(raw.outputLimited).toBe(true);
    expect(raw.truncated).toBe(true);
    expect(Buffer.byteLength(raw.stdout, 'utf8')).toBeLessThanOrEqual(4096);
  });
});

describe('limits: deep recursion', () => {
  it('reports an error without taking down the host', async () => {
    const r = await executor.run({
      language: 'python',
      source: 'import sys\nsys.setrecursionlimit(100000)\n\ndef f():\n    return f()\n\nf()\n',
      timeoutMs: 5000,
    });
    expect(['error', 'memory_limit']).toContain(r.status);
    expect(r.error?.traceback ?? '').toMatch(/RecursionError|MemoryError/);
  });
});

describe('limits: concurrency cap (429 path)', () => {
  it('sheds load with QueueFullError when saturated', async () => {
    // Pool of 1 active + 1 queued; the 3rd must be rejected.
    const tinyExecutor = new Executor(registry, new ProcessSandbox(), undefined);
    // Use a custom small pool via the WorkerPool directly through Executor.
    const { WorkerPool } = await import('../src/exec/queue');
    const ex = new Executor(registry, new ProcessSandbox(), new WorkerPool(1, 1));
    const slow = () =>
      ex.run({ language: 'python', source: 'import time\ntime.sleep(1.5)\nprint("done")\n', timeoutMs: 5000 });
    const p1 = slow();
    const p2 = slow();
    const p3 = slow();
    const settled = await Promise.allSettled([p1, p2, p3]);
    const rejected = settled.filter((s) => s.status === 'rejected');
    expect(rejected.length).toBeGreaterThanOrEqual(1);
    expect((rejected[0] as PromiseRejectedResult).reason).toHaveProperty('name', 'QueueFullError');
    // Drain so the pool processes can finish before the suite exits.
    await Promise.allSettled([p1, p2, p3]);
    void tinyExecutor;
  });
});
