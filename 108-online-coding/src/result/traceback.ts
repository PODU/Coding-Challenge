import type { ErrorType, Language } from '../types';

export interface ParsedError {
  type: ErrorType;
  message: string;
  line: number | null;
}

/**
 * Parse interpreter stderr into {type, message, line}. Never throws; anything
 * unrecognised becomes a generic runtime error.
 */
export function parseStderr(language: Language, stderr: string): ParsedError {
  const text = stderr.trimEnd();
  if (language === 'python') return parsePython(text);
  return parseNode(text);
}

const PY_FILE_LINE = /File "(?:.*?)", line (\d+)/g;

function parsePython(stderr: string): ParsedError {
  // SyntaxError (incl. IndentationError) is reported without a runtime
  // "Traceback" frame, or with a caret block; classify as syntax.
  const isSyntax = /\b(SyntaxError|IndentationError|TabError)\b/.test(stderr);

  // The last "line N" reference is the deepest frame (the actual fault site).
  let line: number | null = null;
  let m: RegExpExecArray | null;
  while ((m = PY_FILE_LINE.exec(stderr)) !== null) {
    line = Number.parseInt(m[1]!, 10);
  }
  PY_FILE_LINE.lastIndex = 0;

  // The final non-empty line is typically "<ErrorName>: <message>".
  const lines = stderr.split('\n').filter((l) => l.trim() !== '');
  const lastLine = lines.length > 0 ? lines[lines.length - 1]!.trim() : '';
  const message = lastLine || 'Execution failed';

  return {
    type: isSyntax ? 'syntax' : 'runtime',
    message,
    line,
  };
}

const NODE_SYNTAX_LINE = /:(\d+)\r?\n/;

function parseNode(stderr: string): ParsedError {
  const isSyntax = /\bSyntaxError\b/.test(stderr);
  let line: number | null = null;

  // Node syntax errors: "/path/program.js:3\n  <code>\n ... SyntaxError: ..."
  const headMatch = stderr.match(NODE_SYNTAX_LINE);
  if (headMatch && headMatch[1]) line = Number.parseInt(headMatch[1], 10);

  // Runtime errors: "    at Object.<anonymous> (/path/program.js:5:13)"
  if (line === null) {
    const atMatches = [...stderr.matchAll(/program\.js:(\d+):\d+/g)];
    const first = atMatches[0];
    if (first && first[1]) {
      line = Number.parseInt(first[1], 10);
    }
  }

  // The "<ErrorName>: <message>" line.
  const errLine = stderr
    .split('\n')
    .map((l) => l.trim())
    .find((l) => /^[A-Z][A-Za-z]*Error\b/.test(l) || /Error:/.test(l));

  const message = errLine ?? (stderr.split('\n').find((l) => l.trim() !== '') ?? 'Execution failed');

  return {
    type: isSyntax ? 'syntax' : 'runtime',
    message,
    line,
  };
}
