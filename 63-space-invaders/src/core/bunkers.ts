import type { GameConfig } from './config.js';
import { intersects, type Bunker, type Projectile, type Rect } from './types.js';
import { projectileRect } from './types.js';

export function createBunkers(config: GameConfig): Bunker[] {
  const b = config.bunkers;
  const bunkers: Bunker[] = [];
  const bunkerW = b.cols * b.cellSize.x;
  const spacing = config.bounds.w / b.count;
  for (let i = 0; i < b.count; i++) {
    const cx = spacing * (i + 0.5);
    bunkers.push({
      pos: { x: config.bounds.x + cx - bunkerW / 2, y: b.y },
      cols: b.cols,
      rows: b.rows,
      cellSize: { x: b.cellSize.x, y: b.cellSize.y },
      cells: Array.from({ length: b.rows }, () => new Array<boolean>(b.cols).fill(true)),
    });
  }
  return bunkers;
}

export function bunkerRect(bunker: Bunker): Rect {
  return {
    x: bunker.pos.x,
    y: bunker.pos.y,
    w: bunker.cols * bunker.cellSize.x,
    h: bunker.rows * bunker.cellSize.y,
  };
}

/** The bunker column under the projectile's centre. */
function impactColumn(bunker: Bunker, p: Projectile): number {
  const px = p.pos.x + p.w / 2;
  let col = Math.floor((px - bunker.pos.x) / bunker.cellSize.x);
  if (col < 0) col = 0;
  if (col >= bunker.cols) col = bunker.cols - 1;
  return col;
}

/**
 * Knock one cell out of the bunker where the projectile hits it: the lowest
 * intact cell for player shots (fromBelow), the highest for bombs. Returns the
 * cell, or null if nothing was hit.
 */
export function erodeBunker(
  bunker: Bunker,
  p: Projectile,
  fromBelow: boolean,
): { x: number; y: number } | null {
  if (!intersects(projectileRect(p), bunkerRect(bunker))) return null;
  const col = impactColumn(bunker, p);

  if (fromBelow) {
    for (let r = bunker.rows - 1; r >= 0; r--) {
      if (bunker.cells[r][col]) {
        bunker.cells[r][col] = false;
        return { x: col, y: r };
      }
    }
  } else {
    for (let r = 0; r < bunker.rows; r++) {
      if (bunker.cells[r][col]) {
        bunker.cells[r][col] = false;
        return { x: col, y: r };
      }
    }
  }
  return null; // the column is already shot through
}
