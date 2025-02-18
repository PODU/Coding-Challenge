/**
 * Splits Markdown into slides at lines of three or more dashes. A `---` inside a fenced
 * code block is left alone. Front matter must already have been removed.
 */
import type { RawSlide } from './slide-model.js';

const SEPARATOR = /^-{3,}\s*$/;
// Up to 3 spaces of indent, 3+ backticks or tildes, then an optional info string.
const FENCE_OPEN = /^(\s{0,3})(`{3,}|~{3,})(.*)$/;

interface FenceState {
  char: '`' | '~';
  length: number;
}

// A closing fence uses the same character, is at least as long, and has no info string.
function isClosingFence(line: string, fence: FenceState): boolean {
  const match = FENCE_OPEN.exec(line);
  if (!match) return false;
  const marker = match[2]!;
  const info = match[3]!;
  return marker[0] === fence.char && marker.length >= fence.length && info.trim() === '';
}

/**
 * @param body Markdown with front matter removed and `\n` line endings.
 * @param lineOffset Lines in the file before `body` starts, so `startLine` matches the file.
 */
export function splitSlides(body: string, lineOffset = 0): RawSlide[] {
  if (body === '') return [];

  const lines = body.split('\n');
  const slides: RawSlide[] = [];

  let current: string[] = [];
  let chunkStartLineIdx = 0;
  let fence: FenceState | null = null;

  const pushChunk = (): void => {
    // Point startLine at the first non-blank line, or the first line if all are blank.
    let contentOffset = current.findIndex((l) => l.trim() !== '');
    if (contentOffset < 0) contentOffset = 0;
    slides.push({
      index: slides.length,
      markdown: current.join('\n'),
      startLine: lineOffset + chunkStartLineIdx + contentOffset + 1,
    });
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;

    if (fence) {
      if (isClosingFence(line, fence)) fence = null;
      current.push(line);
      continue;
    }

    const fenceOpen = FENCE_OPEN.exec(line);
    if (fenceOpen) {
      const marker = fenceOpen[2]!;
      fence = { char: marker[0] as '`' | '~', length: marker.length };
      current.push(line);
      continue;
    }

    if (SEPARATOR.test(line)) {
      pushChunk();
      current = [];
      chunkStartLineIdx = i + 1;
      continue;
    }

    current.push(line);
  }

  // There is always a last chunk, even if it is empty; buildDeck drops empty slides.
  pushChunk();

  return slides;
}
