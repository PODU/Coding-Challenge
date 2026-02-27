/**
 * Output comparison for problem mode. Normalizes trailing whitespace on each
 * line and trailing blank lines so cosmetic differences don't fail a case.
 */
export function normalizeOutput(s: string): string {
  return s
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[ \t]+$/g, ''))
    .join('\n')
    .replace(/\n+$/g, '');
}

export function outputsMatch(expected: string, actual: string): boolean {
  return normalizeOutput(expected) === normalizeOutput(actual);
}
