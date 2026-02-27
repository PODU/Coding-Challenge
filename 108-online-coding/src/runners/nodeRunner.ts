import { config } from '../config';
import type { Language } from '../types';
import { baseScrubbedEnv, type BuiltCommand, type Runner } from './runner';

/**
 * Runs untrusted JavaScript via the bundled Node binary.
 *
 *   node program.js
 *
 * The program file lives in an isolated temp dir; with no node_modules present
 * and a scrubbed env, attacker code cannot resolve project modules.
 */
export class NodeRunner implements Runner {
  readonly id: Language = 'javascript';
  readonly label = 'JavaScript (Node 20)';
  readonly fileName = 'program.js';

  buildCommand(programPath: string): BuiltCommand {
    // --no-warnings keeps stderr clean; the temp cwd has no node_modules.
    return { command: config.nodeBin, args: ['--no-warnings', programPath] };
  }

  scrubbedEnv(): NodeJS.ProcessEnv {
    // Prevent NODE_OPTIONS / NODE_PATH injection from the host environment.
    return baseScrubbedEnv({ NODE_OPTIONS: '', NODE_PATH: '' });
  }

  detectVersion(): Promise<string> {
    return Promise.resolve(process.versions.node);
  }
}
