import { describe, it, expect } from 'vitest';
import { Executor } from '../src/exec/executor';
import { defaultRegistry } from '../src/runners/registry';
import { gradeSubmission } from '../src/problems/grader';
import type { Problem } from '../src/types';

const executor = new Executor(defaultRegistry());

const sumProblem: Problem = {
  id: 'sum-of-ints',
  title: 'Sum',
  statementMarkdown: '',
  languages: ['python'],
  starterCode: {},
  testCases: [
    { name: 'a', stdin: '1 2 3 4\n', expectedStdout: '10\n', hidden: false },
    { name: 'b', stdin: '42\n', expectedStdout: '42\n', hidden: false },
    { name: 'h1', stdin: '-5 5\n', expectedStdout: '0\n', hidden: true },
    { name: 'h2', stdin: '\n', expectedStdout: '0\n', hidden: true },
  ],
};

const CORRECT = 'import sys\nprint(sum(int(x) for x in sys.stdin.read().split()))\n';
const ALWAYS_ZERO = 'print(0)\n';

describe('problem mode: submit', () => {
  it('correct solution passes all cases', async () => {
    const res = await gradeSubmission(executor, sumProblem, 'python', CORRECT);
    expect(res.passed).toBe(true);
    expect(res.passedCases).toBe(res.totalCases);
    expect(res.totalCases).toBe(4);
  });

  it('wrong solution reports correct passed count and verdicts', async () => {
    const res = await gradeSubmission(executor, sumProblem, 'python', ALWAYS_ZERO);
    expect(res.passed).toBe(false);
    // print(0) only matches the two cases whose sum is 0 (h1, h2).
    expect(res.passedCases).toBe(2);
  });

  it('hidden cases redact expected/actual; public cases reveal them', async () => {
    const res = await gradeSubmission(executor, sumProblem, 'python', ALWAYS_ZERO);
    const pub = res.results.find((r) => r.name === 'a')!;
    const hid = res.results.find((r) => r.name === 'h1')!;
    expect(pub.hidden).toBe(false);
    expect(pub.expected).toBeDefined();
    expect(pub.actual).toBeDefined();
    expect(hid.hidden).toBe(true);
    expect(hid.expected).toBeUndefined();
    expect(hid.actual).toBeUndefined();
  });

  it('a case that times out is reported as timeout, a passing case still passes', async () => {
    // The program loops forever only when stdin is "loop"; otherwise prints ok.
    const timeoutProblem: Problem = {
      id: 't',
      title: 'T',
      statementMarkdown: '',
      languages: ['python'],
      starterCode: {},
      testCases: [
        { name: 'fast', stdin: 'go\n', expectedStdout: 'ok\n', hidden: false, timeoutMs: 1500 },
        { name: 'loops', stdin: 'loop\n', expectedStdout: 'never\n', hidden: false, timeoutMs: 700 },
      ],
    };
    const src =
      'import sys\n' +
      'if sys.stdin.readline().strip() == "loop":\n' +
      '    while True:\n        pass\n' +
      'print("ok")\n';
    const res = await gradeSubmission(executor, timeoutProblem, 'python', src);
    const fast = res.results.find((r) => r.name === 'fast')!;
    const loops = res.results.find((r) => r.name === 'loops')!;
    expect(fast.status).toBe('pass');
    expect(loops.status).toBe('timeout');
  });
});
