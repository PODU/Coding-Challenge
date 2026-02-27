import type { Language, LanguageInfo } from '../types';
import { NodeRunner } from './nodeRunner';
import { PythonRunner } from './pythonRunner';
import type { Runner } from './runner';

/**
 * The runner registry. Resolving a language to a runner is the single point
 * that defines which languages the platform supports.
 */
export class RunnerRegistry {
  private readonly runners = new Map<Language, Runner>();

  constructor(runners: Runner[]) {
    for (const r of runners) this.runners.set(r.id, r);
  }

  has(language: string): language is Language {
    return this.runners.has(language as Language);
  }

  get(language: Language): Runner {
    const r = this.runners.get(language);
    if (!r) throw new Error(`No runner registered for language: ${language}`);
    return r;
  }

  list(): Runner[] {
    return [...this.runners.values()];
  }

  async languageInfos(): Promise<LanguageInfo[]> {
    return Promise.all(
      this.list().map(async (r) => ({
        id: r.id,
        label: r.label,
        version: await r.detectVersion(),
      })),
    );
  }
}

export function defaultRegistry(): RunnerRegistry {
  return new RunnerRegistry([new PythonRunner(), new NodeRunner()]);
}
