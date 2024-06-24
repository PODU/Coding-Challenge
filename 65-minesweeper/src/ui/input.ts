import type { Difficulty, DifficultyName } from '../core/difficulty';
import { PRESETS } from '../core/difficulty';

export type Command =
  | { kind: 'reveal'; r: number; c: number }
  | { kind: 'flag'; r: number; c: number }
  | { kind: 'new'; difficulty: Difficulty }
  | { kind: 'help' }
  | { kind: 'quit' }
  | { kind: 'noop' }
  | { kind: 'error'; message: string };

function parseInt10(s: string): number | null {
  if (!/^-?\d+$/.test(s)) return null;
  return Number.parseInt(s, 10);
}

/**
 * Parses one typed command. Accepted forms (case-insensitive):
 *   r <row> <col> | reveal <row> <col>
 *   f <row> <col> | flag <row> <col>
 *   new <beginner|intermediate|expert>
 *   new custom <rows> <cols> <mines>
 *   help | h | ?
 *   quit | q | exit
 */
export function parseCommand(line: string): Command {
  const trimmed = line.trim();
  if (trimmed === '') return { kind: 'noop' };

  const parts = trimmed.split(/\s+/);
  const verb = parts[0].toLowerCase();

  switch (verb) {
    case 'r':
    case 'reveal':
    case 'f':
    case 'flag': {
      if (parts.length !== 3) {
        return { kind: 'error', message: `Usage: ${verb} <row> <col>` };
      }
      const r = parseInt10(parts[1]);
      const c = parseInt10(parts[2]);
      if (r === null || c === null) {
        return { kind: 'error', message: 'Row and column must be integers.' };
      }
      const kind = verb === 'r' || verb === 'reveal' ? 'reveal' : 'flag';
      return { kind, r, c };
    }

    case 'new':
    case 'n': {
      if (parts.length === 1) {
        return { kind: 'error', message: 'Usage: new <beginner|intermediate|expert|custom ...>' };
      }
      const name = parts[1].toLowerCase() as DifficultyName;
      if (name === 'custom') {
        if (parts.length !== 5) {
          return { kind: 'error', message: 'Usage: new custom <rows> <cols> <mines>' };
        }
        const rows = parseInt10(parts[2]);
        const cols = parseInt10(parts[3]);
        const mines = parseInt10(parts[4]);
        if (rows === null || cols === null || mines === null) {
          return { kind: 'error', message: 'Custom rows, cols and mines must be integers.' };
        }
        return { kind: 'new', difficulty: { rows, cols, mines } };
      }
      if (name === 'beginner' || name === 'intermediate' || name === 'expert') {
        return { kind: 'new', difficulty: { ...PRESETS[name] } };
      }
      return { kind: 'error', message: `Unknown difficulty: ${parts[1]}` };
    }

    case 'help':
    case 'h':
    case '?':
      return { kind: 'help' };

    case 'quit':
    case 'q':
    case 'exit':
      return { kind: 'quit' };

    default:
      return { kind: 'error', message: `Unknown command: ${verb} (type 'help')` };
  }
}
