import type { Executor } from '../exec/executor';
import { outputsMatch } from '../result/comparator';
import type {
  CaseResult,
  CaseStatus,
  Language,
  Problem,
  RunResult,
  SubmitResult,
} from '../types';

/**
 * Run a submission against every test case of a problem and build a SubmitResult.
 * For hidden cases only the pass/fail verdict is returned, never the output.
 */
export async function gradeSubmission(
  executor: Executor,
  problem: Problem,
  language: Language,
  source: string,
): Promise<SubmitResult> {
  const results: CaseResult[] = [];
  let passedCases = 0;

  for (const testCase of problem.testCases) {
    const run: RunResult = await executor.run({
      language,
      source,
      stdin: testCase.stdin,
      timeoutMs: testCase.timeoutMs,
    });

    const status = caseStatus(run, testCase.expectedStdout);
    if (status === 'pass') passedCases += 1;

    const result: CaseResult = {
      name: testCase.name,
      hidden: testCase.hidden,
      status,
      timeMs: run.timeMs,
      error: run.error,
    };

    // Only reveal expected/actual for non-hidden cases.
    if (!testCase.hidden) {
      result.expected = testCase.expectedStdout;
      result.actual = run.stdout;
    } else {
      result.error = null; // don't leak internals for hidden cases either
    }

    results.push(result);
  }

  return {
    problemId: problem.id,
    passed: passedCases === problem.testCases.length && problem.testCases.length > 0,
    totalCases: problem.testCases.length,
    passedCases,
    results,
  };
}

function caseStatus(run: RunResult, expected: string): CaseStatus {
  switch (run.status) {
    case 'timeout':
      return 'timeout';
    case 'output_limit':
      return 'output_limit';
    case 'ok':
      return outputsMatch(expected, run.stdout) ? 'pass' : 'fail';
    case 'error':
    case 'memory_limit':
    case 'internal_error':
    default:
      return 'error';
  }
}
