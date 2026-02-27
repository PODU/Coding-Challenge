import { execFile } from 'node:child_process';
import { config } from '../config';
import type { Language } from '../types';
import { baseScrubbedEnv, type BuiltCommand, type Runner } from './runner';

/**
 * Runs untrusted Python via the installed CPython.
 *
 *   python -I -B -E program.py
 *
 *   -I  isolated mode: ignore PYTHON* env, user site-packages, and the cwd on
 *       sys.path[0] => no accidental import of an attacker-dropped module.
 *   -B  do not write .pyc files.
 *   -E  ignore PYTHON* environment variables (belt-and-braces with -I).
 */
export class PythonRunner implements Runner {
  readonly id: Language = 'python';
  readonly label = 'Python 3.12';
  readonly fileName = 'program.py';

  buildCommand(programPath: string): BuiltCommand {
    return { command: config.pythonBin, args: ['-I', '-B', '-E', programPath] };
  }

  scrubbedEnv(): NodeJS.ProcessEnv {
    return baseScrubbedEnv({ PYTHONIOENCODING: 'utf-8', PYTHONDONTWRITEBYTECODE: '1' });
  }

  detectVersion(): Promise<string> {
    return new Promise((resolve) => {
      execFile(config.pythonBin, ['--version'], { timeout: 5000 }, (err, stdout, stderr) => {
        if (err) {
          resolve('unknown');
          return;
        }
        const out = `${stdout} ${stderr}`.trim();
        const m = out.match(/(\d+\.\d+\.\d+)/);
        resolve(m?.[1] ?? out ?? 'unknown');
      });
    });
  }
}
