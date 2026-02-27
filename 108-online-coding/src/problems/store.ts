import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { isLanguage, type Problem, type PublicProblem, type TestCase } from '../types';

/** In-memory, read-only problem store loaded once at boot. */
export class ProblemStore {
  private readonly byId = new Map<string, Problem>();

  add(problem: Problem): void {
    this.byId.set(problem.id, problem);
  }

  get(id: string): Problem | undefined {
    return this.byId.get(id);
  }

  list(): Array<Pick<Problem, 'id' | 'title' | 'languages'>> {
    return [...this.byId.values()].map(({ id, title, languages }) => ({ id, title, languages }));
  }

  size(): number {
    return this.byId.size;
  }

  static async fromDir(dir: string): Promise<ProblemStore> {
    const store = new ProblemStore();
    let entries: string[] = [];
    try {
      entries = await readdir(dir);
    } catch {
      return store; // no problems dir -> empty store (still valid)
    }
    for (const entry of entries) {
      if (!entry.endsWith('.json')) continue;
      const raw = await readFile(join(dir, entry), 'utf8');
      const parsed = JSON.parse(raw) as unknown;
      const problem = validateProblem(parsed, entry);
      store.add(problem);
    }
    return store;
  }
}

/** Strip hidden cases and expected output before sending to the client. */
export function toPublicProblem(problem: Problem): PublicProblem {
  return {
    id: problem.id,
    title: problem.title,
    statementMarkdown: problem.statementMarkdown,
    languages: problem.languages,
    starterCode: problem.starterCode,
    publicTestCaseNames: problem.testCases.filter((c) => !c.hidden).map((c) => c.name),
    totalCases: problem.testCases.length,
  };
}

function validateProblem(value: unknown, file: string): Problem {
  if (typeof value !== 'object' || value === null) {
    throw new Error(`Invalid problem file ${file}: not an object`);
  }
  const v = value as Record<string, unknown>;
  if (typeof v.id !== 'string' || typeof v.title !== 'string') {
    throw new Error(`Invalid problem file ${file}: missing id/title`);
  }
  if (!Array.isArray(v.languages) || !v.languages.every(isLanguage)) {
    throw new Error(`Invalid problem file ${file}: bad languages`);
  }
  if (!Array.isArray(v.testCases)) {
    throw new Error(`Invalid problem file ${file}: missing testCases`);
  }
  const testCases: TestCase[] = v.testCases.map((c, i) => {
    const tc = c as Record<string, unknown>;
    if (typeof tc.stdin !== 'string' || typeof tc.expectedStdout !== 'string') {
      throw new Error(`Invalid problem file ${file}: testCase[${i}] missing stdin/expectedStdout`);
    }
    return {
      name: typeof tc.name === 'string' ? tc.name : `case-${i + 1}`,
      stdin: tc.stdin,
      expectedStdout: tc.expectedStdout,
      hidden: tc.hidden === true,
      timeoutMs: typeof tc.timeoutMs === 'number' ? tc.timeoutMs : undefined,
    };
  });
  return {
    id: v.id,
    title: v.title,
    statementMarkdown: typeof v.statementMarkdown === 'string' ? v.statementMarkdown : '',
    languages: v.languages,
    starterCode: (v.starterCode as Problem['starterCode']) ?? {},
    testCases,
  };
}
