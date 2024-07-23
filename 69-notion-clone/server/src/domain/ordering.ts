/**
 * Fractional sibling ordering.
 *
 * Siblings carry a REAL `position`. Inserting between neighbours A and B uses
 * the midpoint so a single insert/reorder rewrites no other rows. When the
 * midpoint underflows float precision we renormalise the whole sibling group
 * to 1, 2, 3 … (within the caller's transaction) and retry.
 */
export interface Positioned {
  id: string;
  position: number;
}

export function midpoint(prev: number | null, next: number | null): number {
  if (prev === null && next === null) return 1;
  if (prev === null) return next! - 1;
  if (next === null) return prev + 1;
  return (prev + next) / 2;
}

function underflows(
  prev: number | null,
  next: number | null,
  mid: number,
): boolean {
  if (prev !== null && !(mid > prev)) return true;
  if (next !== null && !(mid < next)) return true;
  return false;
}

/**
 * Compute the `position` for an item placed immediately after `afterId` among
 * `siblings` (which must NOT include the item being placed). `afterId === null`
 * places at the start. If the midpoint underflows, `applyRenormalize` is called
 * with the sibling group reassigned to integer positions so the caller can
 * persist them, and a fresh midpoint is returned.
 *
 * @throws if `afterId` is given but not present among `siblings`.
 */
export function positionAfter<T extends Positioned>(
  siblings: T[],
  afterId: string | null,
  applyRenormalize: (renumbered: T[]) => void,
): number {
  const sorted = [...siblings].sort((a, b) => a.position - b.position);

  let prevIdx: number;
  if (afterId === null) {
    prevIdx = -1;
  } else {
    prevIdx = sorted.findIndex((s) => s.id === afterId);
    if (prevIdx === -1) {
      throw new Error(`afterId ${afterId} is not a sibling`);
    }
  }

  const prev = prevIdx >= 0 ? sorted[prevIdx].position : null;
  const next = prevIdx + 1 < sorted.length ? sorted[prevIdx + 1].position : null;
  const mid = midpoint(prev, next);

  if (!underflows(prev, next, mid)) return mid;

  // Renormalise: 1..n on the sorted group, then place between integer neighbours.
  const renumbered = sorted.map((s, i) => ({ ...s, position: i + 1 }));
  applyRenormalize(renumbered);
  const newPrev = prevIdx >= 0 ? prevIdx + 1 : null;
  const newNext = prevIdx + 1 < renumbered.length ? prevIdx + 2 : null;
  return midpoint(newPrev, newNext);
}

/** Position that appends after the last sibling (or 1 if none). */
export function positionAtEnd(siblings: Positioned[]): number {
  if (siblings.length === 0) return 1;
  return Math.max(...siblings.map((s) => s.position)) + 1;
}
