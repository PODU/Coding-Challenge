import { config } from '../config';
import type { BuiltCommand } from '../runners/runner';

/**
 * On POSIX, runs the interpreter through `/bin/sh -c` with resource limits:
 *   ulimit -t  CPU seconds   (backstop for the wall-clock timer)
 *   ulimit -v  virtual mem   (big allocations, deep recursion)
 *   ulimit -u  max processes (fork bombs)
 *   ulimit -f  max file size (filling the disk)
 *
 * Windows has no ulimit, so there it returns the command unchanged and only
 * the sandbox's timeout, output cap and tree-kill apply.
 */
export function maybeWrapWithUlimit(built: BuiltCommand): BuiltCommand {
  if (process.platform === 'win32') return built;

  const cpuSeconds = Math.max(1, Math.ceil(config.maxTimeoutMs / 1000) + 1);
  const addressKb = 512 * 1024; // 512 MB virtual address space
  const maxProcs = 64;
  const maxFileKb = 8 * 1024; // 8 MB max single file write

  const quoted = [built.command, ...built.args].map(shQuote).join(' ');
  const preamble =
    `ulimit -t ${cpuSeconds}; ulimit -v ${addressKb}; ` +
    `ulimit -u ${maxProcs}; ulimit -f ${maxFileKb}; `;

  return {
    command: '/bin/sh',
    args: ['-c', `${preamble}exec ${quoted}`],
  };
}

function shQuote(arg: string): string {
  if (/^[A-Za-z0-9_./-]+$/.test(arg)) return arg;
  return `'${arg.replace(/'/g, `'\\''`)}'`;
}
