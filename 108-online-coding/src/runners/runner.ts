import type { Language } from '../types';

export interface BuiltCommand {
  command: string;
  args: string[];
}

/**
 * A Runner knows how to materialize and launch one language. Adding a language
 * = adding a Runner. Runners don't execute anything; they only describe how
 * the Sandbox should spawn the program.
 */
export interface Runner {
  readonly id: Language;
  readonly label: string;
  /** Source file name written into the per-run temp dir. */
  readonly fileName: string;

  /** Build the spawn argv given the absolute path to the program file. */
  buildCommand(programPath: string): BuiltCommand;

  /** A scrubbed environment for the child (no host secrets / module paths). */
  scrubbedEnv(): NodeJS.ProcessEnv;

  /** Detect the installed toolchain version (best-effort, cached by caller). */
  detectVersion(): Promise<string>;
}

/**
 * A minimal, scrubbed environment. We deliberately drop the host env so the
 * child cannot read secrets, and keep only what an interpreter needs to start.
 */
export function baseScrubbedEnv(extra: NodeJS.ProcessEnv = {}): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {
    PATH: process.env.PATH ?? '',
    // Force deterministic, sandbox-friendly behavior.
    LANG: 'C.UTF-8',
    LC_ALL: 'C.UTF-8',
    ...extra,
  };
  if (process.platform === 'win32') {
    // Windows needs SystemRoot/TEMP for the CRT + interpreter to initialize.
    env.SystemRoot = process.env.SystemRoot ?? process.env.WINDIR ?? '';
    env.TEMP = process.env.TEMP ?? '';
    env.TMP = process.env.TMP ?? '';
    env.PATHEXT = process.env.PATHEXT ?? '';
  }
  return env;
}
