/**
 * Shared, type-checked contracts used end-to-end (API, runner, reporter, web).
 */

export type Language = 'python' | 'javascript';

export const LANGUAGES: readonly Language[] = ['python', 'javascript'] as const;

export function isLanguage(value: unknown): value is Language {
  return typeof value === 'string' && (LANGUAGES as readonly string[]).includes(value);
}

/** Top-level status of a single run. */
export type RunStatus =
  | 'ok'
  | 'error'
  | 'timeout'
  | 'output_limit'
  | 'memory_limit'
  | 'internal_error';

/** Classification of an error for the UI. */
export type ErrorType =
  | 'syntax'
  | 'runtime'
  | 'timeout'
  | 'output_limit'
  | 'memory_limit'
  | 'internal'
  | 'validation';

export interface ErrorInfo {
  type: ErrorType;
  message: string;
  /** 1-based source line parsed from a traceback, when available. */
  line: number | null;
  /** Full traceback / stderr excerpt, when available. */
  traceback: string | null;
}

/** Structured, machine-readable result of one execution. */
export interface RunResult {
  status: RunStatus;
  exitCode: number | null;
  /** Signal that terminated the process, if any (e.g. 'SIGKILL'). */
  signal: string | null;
  stdout: string;
  stderr: string;
  timeMs: number;
  /** true when stdout/stderr hit the byte ceiling and capture was stopped. */
  truncated: boolean;
  error: ErrorInfo | null;
}

/** Raw outcome from the sandbox, before reporter normalization. */
export interface RawExecution {
  exitCode: number | null;
  signal: string | null;
  stdout: string;
  stderr: string;
  timeMs: number;
  truncated: boolean;
  timedOut: boolean;
  outputLimited: boolean;
  /** true when the runner/sandbox itself failed (e.g. interpreter not found). */
  internalError: boolean;
  internalMessage?: string;
}

// ---------------------------------------------------------------------------
// Problem mode
// ---------------------------------------------------------------------------

export interface TestCase {
  name: string;
  stdin: string;
  expectedStdout: string;
  hidden: boolean;
  timeoutMs?: number;
}

export interface Problem {
  id: string;
  title: string;
  statementMarkdown: string;
  languages: Language[];
  starterCode: Partial<Record<Language, string>>;
  testCases: TestCase[];
}

/** Public view of a problem — hidden testcases stripped. */
export interface PublicProblem {
  id: string;
  title: string;
  statementMarkdown: string;
  languages: Language[];
  starterCode: Partial<Record<Language, string>>;
  /** Only names of non-hidden cases are exposed. */
  publicTestCaseNames: string[];
  totalCases: number;
}

export type CaseStatus = 'pass' | 'fail' | 'timeout' | 'error' | 'output_limit';

export interface CaseResult {
  name: string;
  hidden: boolean;
  status: CaseStatus;
  timeMs: number;
  /** Present only for non-hidden cases. */
  expected?: string;
  /** Present only for non-hidden cases. */
  actual?: string;
  error: ErrorInfo | null;
}

export interface SubmitResult {
  problemId: string;
  passed: boolean;
  totalCases: number;
  passedCases: number;
  results: CaseResult[];
}

export interface LanguageInfo {
  id: Language;
  label: string;
  version: string;
}
