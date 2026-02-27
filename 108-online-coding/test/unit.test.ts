import { describe, it, expect } from 'vitest';
import { defaultRegistry } from '../src/runners/registry';
import { buildRunResult } from '../src/result/reporter';
import { parseStderr } from '../src/result/traceback';
import { normalizeOutput, outputsMatch } from '../src/result/comparator';
import { runRequestSchema, isSizeViolation } from '../src/api/schemas';
import type { RawExecution } from '../src/types';

function raw(over: Partial<RawExecution>): RawExecution {
  return {
    exitCode: 0,
    signal: null,
    stdout: '',
    stderr: '',
    timeMs: 1,
    truncated: false,
    timedOut: false,
    outputLimited: false,
    internalError: false,
    ...over,
  };
}

describe('runner registry', () => {
  const reg = defaultRegistry();
  it('resolves known languages with correct file + argv', () => {
    expect(reg.has('python')).toBe(true);
    expect(reg.has('javascript')).toBe(true);
    expect(reg.get('python').fileName).toBe('program.py');
    const cmd = reg.get('python').buildCommand('/tmp/program.py');
    expect(cmd.args).toContain('-I');
    expect(cmd.args).toContain('/tmp/program.py');
  });
  it('rejects unknown languages', () => {
    expect(reg.has('ruby')).toBe(false);
    expect(() => reg.get('ruby' as never)).toThrow();
  });
});

describe('reporter status mapping', () => {
  it('exit 0 -> ok', () => {
    const r = buildRunResult('python', raw({ exitCode: 0, stdout: 'hi\n' }), 5000);
    expect(r.status).toBe('ok');
    expect(r.error).toBeNull();
    expect(r.stdout).toBe('hi\n');
  });
  it('non-zero exit with stderr -> error with parsed info', () => {
    const stderr =
      'Traceback (most recent call last):\n  File "program.py", line 3, in <module>\nNameError: name \'y\' is not defined\n';
    const r = buildRunResult('python', raw({ exitCode: 1, stderr }), 5000);
    expect(r.status).toBe('error');
    expect(r.error?.type).toBe('runtime');
    expect(r.error?.line).toBe(3);
    expect(r.error?.message).toContain('NameError');
  });
  it('timeout -> timeout status', () => {
    const r = buildRunResult('python', raw({ timedOut: true, exitCode: null, signal: 'SIGKILL' }), 800);
    expect(r.status).toBe('timeout');
    expect(r.error?.type).toBe('timeout');
  });
  it('output limit -> output_limit status', () => {
    const r = buildRunResult('python', raw({ outputLimited: true, truncated: true }), 5000);
    expect(r.status).toBe('output_limit');
    expect(r.truncated).toBe(true);
  });
  it('internal error -> internal_error status', () => {
    const r = buildRunResult('python', raw({ internalError: true, internalMessage: 'ENOENT' }), 5000);
    expect(r.status).toBe('internal_error');
    expect(r.error?.type).toBe('internal');
  });
});

describe('traceback parser', () => {
  it('parses a Python SyntaxError as syntax', () => {
    const stderr =
      '  File "program.py", line 1\n    print("x"\n            ^\nSyntaxError: \'(\' was never closed\n';
    const p = parseStderr('python', stderr);
    expect(p.type).toBe('syntax');
    expect(p.line).toBe(1);
    expect(p.message).toContain('SyntaxError');
  });
  it('parses ZeroDivisionError line', () => {
    const stderr =
      'Traceback (most recent call last):\n  File "program.py", line 1, in <module>\nZeroDivisionError: division by zero\n';
    const p = parseStderr('python', stderr);
    expect(p.type).toBe('runtime');
    expect(p.line).toBe(1);
    expect(p.message).toContain('ZeroDivisionError');
  });
  it('parses a Node SyntaxError line', () => {
    const stderr = '/tmp/program.js:2\nconst = 1\n      ^\n\nSyntaxError: Unexpected token \'=\'\n';
    const p = parseStderr('javascript', stderr);
    expect(p.type).toBe('syntax');
    expect(p.line).toBe(2);
  });
});

describe('comparator (problem mode)', () => {
  it('normalizes trailing whitespace and blank lines', () => {
    expect(normalizeOutput('a  \nb\n\n\n')).toBe('a\nb');
    expect(outputsMatch('10\n', '10')).toBe(true);
    expect(outputsMatch('10\n', '11\n')).toBe(false);
  });
});

describe('validation (R4 / size caps)', () => {
  it('rejects empty source', () => {
    const r = runRequestSchema.safeParse({ language: 'python', source: '' });
    expect(r.success).toBe(false);
  });
  it('rejects unknown language', () => {
    const r = runRequestSchema.safeParse({ language: 'ruby', source: 'x' });
    expect(r.success).toBe(false);
  });
  it('flags oversized source as a size violation (413)', () => {
    const big = 'x'.repeat(200 * 1024);
    const r = runRequestSchema.safeParse({ language: 'python', source: big });
    expect(r.success).toBe(false);
    if (!r.success) expect(isSizeViolation(r.error)).toBe(true);
  });
  it('accepts a valid request', () => {
    const r = runRequestSchema.safeParse({ language: 'python', source: 'print(1)' });
    expect(r.success).toBe(true);
  });
});
