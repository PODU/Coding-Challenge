export interface ByteRange {
  start: number;
  end: number;
}

export type ParsedRange = ByteRange | null | 'invalid';

/**
 * Parse a `Range` header for a body of `size` bytes. Returns null when there is
 * no header, 'invalid' when it is malformed or unsatisfiable, and otherwise an
 * inclusive byte range. Only single ranges are supported (`a-b`, `a-`, `-n`).
 */
export function parseRange(header: string | undefined, size: number): ParsedRange {
  if (header === undefined || header === '') {
    return null;
  }
  const match = /^bytes=(\d*)-(\d*)$/.exec(header.trim());
  if (!match) {
    return 'invalid';
  }
  const startText = match[1] ?? '';
  const endText = match[2] ?? '';
  if (startText === '' && endText === '') {
    return 'invalid';
  }

  let start: number;
  let end: number;
  if (startText === '') {
    // "-n" means the last n bytes.
    const n = Number.parseInt(endText, 10);
    if (n <= 0) {
      return 'invalid';
    }
    start = Math.max(0, size - n);
    end = size - 1;
  } else {
    start = Number.parseInt(startText, 10);
    end = endText === '' ? size - 1 : Number.parseInt(endText, 10);
  }

  end = Math.min(end, size - 1);
  if (start < 0 || start > end || start >= size) {
    return 'invalid';
  }
  return { start, end };
}
