/**
 * Line splitting and the comparison keys behind the "ignore ..." options.
 * Only the key is normalized; the displayed text is always the raw line.
 */

export interface IgnoreOptions {
  /** Trim and collapse whitespace, which also hides indentation-only changes. */
  ignoreWhitespace?: boolean;
  ignoreCase?: boolean;
  /** Added/removed blank lines are shown muted and not counted as changes. */
  ignoreBlankLines?: boolean;
  /** Strip a trailing CR so CRLF and LF lines compare equal. Default true. */
  ignoreLineEndings?: boolean;
}

export interface SplitResult {
  lines: string[];
  hadTrailingNewline: boolean;
}

/**
 * Split on '\n', leaving any '\r' on the line. A final newline doesn't create
 * an empty last line; it is recorded instead, for the "\ No newline at end of
 * file" marker.
 */
export function splitLines(text: string): SplitResult {
  if (text === '') return { lines: [], hadTrailingNewline: false };
  const hadTrailingNewline = text.endsWith('\n');
  const body = hadTrailingNewline ? text.slice(0, -1) : text;
  return { lines: body.split('\n'), hadTrailingNewline };
}

export function lineKey(line: string, opts: IgnoreOptions = {}): string {
  let key = line;
  if (opts.ignoreLineEndings !== false) key = key.replace(/\r$/, '');
  if (opts.ignoreWhitespace) key = key.replace(/\s+/g, ' ').trim();
  if (opts.ignoreCase) key = key.toLowerCase();
  return key;
}

export function isBlank(line: string): boolean {
  return /^\s*$/.test(line);
}

/** Strip a trailing CR for display (a bare '\r' renders as nothing anyway). */
export function displayText(line: string): string {
  return line.replace(/\r$/, '');
}
