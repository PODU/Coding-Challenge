import type { RNG } from "./rng.js";
import { IndexRangeError } from "./errors.js";

/** One name per non-blank line, trimmed. Inner spaces are kept. */
export function parseNames(raw: string): string[] {
  return raw
    .split(/\r\n|\r|\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

/** Fisher-Yates shuffle into a new array. */
export function shuffle<T>(xs: readonly T[], rng: RNG): T[] {
  const out = xs.slice();
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(rng.next() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

/** Case-insensitive, numeric-aware sort into a new array. */
export function sortNames(xs: readonly string[]): string[] {
  const collator = new Intl.Collator(undefined, { sensitivity: "base", numeric: true });
  // Tie-break on the original index so equal names keep their order on any engine.
  return xs
    .map((value, index) => ({ value, index }))
    .sort((a, b) => {
      const c = collator.compare(a.value, b.value);
      return c !== 0 ? c : a.index - b.index;
    })
    .map((d) => d.value);
}

export function addName(xs: readonly string[], name: string): string[] {
  return [...xs, name];
}

/** Return a copy of xs without element `index`. */
export function removeAt<T>(xs: readonly T[], index: number): T[] {
  if (!Number.isInteger(index) || index < 0 || index >= xs.length) {
    throw new IndexRangeError(`removeAt: index ${index} out of range [0, ${xs.length})`);
  }
  return xs.slice(0, index).concat(xs.slice(index + 1));
}

/**
 * Drop repeated names, keeping the first. The wheel itself keeps duplicates as
 * separate segments; this is only for display.
 */
export function dedupe(xs: readonly string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const x of xs) {
    if (!seen.has(x)) {
      seen.add(x);
      out.push(x);
    }
  }
  return out;
}
