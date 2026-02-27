import { describe, it, expect } from 'vitest';
import { Executor } from '../src/exec/executor';
import { defaultRegistry } from '../src/runners/registry';

const executor = new Executor(defaultRegistry());

// Python/Node emit CRLF on Windows; normalize so assertions are OS-agnostic.
const lf = (s: string): string => s.replace(/\r\n/g, '\n');

describe('integration: safe Python programs (Steps 1-2)', () => {
  it('Hello World -> stdout, exit 0', async () => {
    const r = await executor.run({ language: 'python', source: 'print("hello")\n' });
    expect(r.status).toBe('ok');
    expect(lf(r.stdout)).toBe('hello\n');
    expect(r.exitCode).toBe(0);
  });

  it('multiline output -> 5 lines', async () => {
    const r = await executor.run({
      language: 'python',
      source: 'for i in range(5):\n    print(i)\n',
    });
    expect(r.status).toBe('ok');
    expect(r.stdout.trim().split('\n')).toHaveLength(5);
  });

  it('math / variables', async () => {
    const r = await executor.run({ language: 'python', source: 'x = 6 * 7\nprint(x)\n' });
    expect(lf(r.stdout)).toBe('42\n');
  });

  it('stdin echo', async () => {
    const r = await executor.run({
      language: 'python',
      source: 'import sys\nprint(sys.stdin.readline().strip())\n',
      stdin: 'echo me\n',
    });
    expect(lf(r.stdout)).toBe('echo me\n');
  });

  it('syntax error -> status error, type syntax, line parsed', async () => {
    const r = await executor.run({ language: 'python', source: 'print("x"\n' });
    expect(r.status).toBe('error');
    expect(r.error?.type).toBe('syntax');
    expect(r.error?.line).toBe(1);
  });

  it('undefined variable -> runtime NameError', async () => {
    const r = await executor.run({ language: 'python', source: 'print(y)\n' });
    expect(r.status).toBe('error');
    expect(r.error?.type).toBe('runtime');
    expect(r.error?.traceback).toContain('NameError');
  });

  it('division by zero -> runtime ZeroDivisionError on line 1', async () => {
    const r = await executor.run({ language: 'python', source: 'print(1/0)\n' });
    expect(r.status).toBe('error');
    expect(r.error?.type).toBe('runtime');
    expect(r.error?.message).toContain('ZeroDivisionError');
    expect(r.error?.line).toBe(1);
  });
});

describe('integration: safe JavaScript programs', () => {
  it('console.log arithmetic', async () => {
    const r = await executor.run({ language: 'javascript', source: 'console.log(1 + 1)\n' });
    expect(r.status).toBe('ok');
    expect(lf(r.stdout)).toBe('2\n');
  });

  it('JS runtime error captured', async () => {
    const r = await executor.run({ language: 'javascript', source: 'notDefined()\n' });
    expect(r.status).toBe('error');
    expect(r.error?.traceback ?? '').toContain('ReferenceError');
  });
});
