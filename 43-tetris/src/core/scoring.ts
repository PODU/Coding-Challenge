// Scoring, levels and gravity speed.
//
// Line clears use the NES table scaled by (level + 1). The challenge brief
// suggests 10/30/50, but the NES values keep the same idea (more lines at once
// is worth disproportionately more) and are what players know.

const LINE_BASE = [0, 40, 100, 300, 1200] as const;

export function lineScore(linesCleared: number, level: number): number {
  if (linesCleared <= 0) return 0;
  const base = LINE_BASE[Math.min(linesCleared, 4)];
  return base * (level + 1);
}

/** One level per 10 lines. */
export function nextLevel(totalLines: number): number {
  return Math.floor(totalLines / 10);
}

// Milliseconds per cell, by level.
const GRAVITY_TABLE = [
  800, 717, 633, 550, 467, 383, 300, 217, 133, 100, 83, 67, 50,
] as const;
const GRAVITY_FLOOR = 50;

export function gravityForLevel(level: number): number {
  if (level <= 0) return GRAVITY_TABLE[0];
  if (level >= GRAVITY_TABLE.length) return GRAVITY_FLOOR;
  return GRAVITY_TABLE[level];
}

/** Points per cell for soft and hard drops. */
export const SOFT_DROP_POINTS = 1;
export const HARD_DROP_POINTS = 2;
