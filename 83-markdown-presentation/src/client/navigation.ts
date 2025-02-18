/**
 * Slide index arithmetic, kept free of the DOM so it can be unit tested.
 * `src/templates/default/client.js` has a copy of the same logic for the browser.
 */

/** Clamp `i` to `[0, count-1]`. An empty deck gives 0. */
export function clampIndex(i: number, count: number): number {
  if (count <= 0) return 0;
  if (Number.isNaN(i)) return 0;
  const n = Math.trunc(i); // Infinity is handled by the checks below
  if (n < 0) return 0;
  if (n > count - 1) return count - 1;
  return n;
}

/** The next slide, stopping at the last one. */
export function nextIndex(i: number, count: number): number {
  return clampIndex(clampIndex(i, count) + 1, count);
}

/** The previous slide, stopping at the first one. */
export function prevIndex(i: number, count: number): number {
  return clampIndex(clampIndex(i, count) - 1, count);
}
