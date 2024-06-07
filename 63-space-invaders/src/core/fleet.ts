import type { GameConfig } from './config.js';
import type { Fleet, GameEvent, Rect } from './types.js';

export function createFleet(config: GameConfig, wave: number): Fleet {
  const f = config.fleet;
  return {
    cols: f.cols,
    rows: f.rows,
    alive: Array.from({ length: f.rows }, () => new Array<boolean>(f.cols).fill(true)),
    origin: {
      x: f.startOrigin.x,
      y: f.startOrigin.y + (wave - 1) * f.waveDescend,
    },
    cellSize: { x: f.cellSize.x, y: f.cellSize.y },
    gap: { x: f.gap.x, y: f.gap.y },
    dir: 1,
    speed: f.baseSpeed * (1 + (wave - 1) * f.waveSpeedup),
    stepDown: f.stepDown,
    bombTimer: 0,
  };
}

export function colStride(fleet: Fleet): number {
  return fleet.cellSize.x + fleet.gap.x;
}

export function rowStride(fleet: Fleet): number {
  return fleet.cellSize.y + fleet.gap.y;
}

export function alienRect(fleet: Fleet, row: number, col: number): Rect {
  return {
    x: fleet.origin.x + col * colStride(fleet),
    y: fleet.origin.y + row * rowStride(fleet),
    w: fleet.cellSize.x,
    h: fleet.cellSize.y,
  };
}

export function aliveCount(fleet: Fleet): number {
  let n = 0;
  for (let r = 0; r < fleet.rows; r++)
    for (let c = 0; c < fleet.cols; c++) if (fleet.alive[r][c]) n++;
  return n;
}

/** Columns (left to right) containing at least one alive alien. */
export function aliveColumns(fleet: Fleet): number[] {
  const cols: number[] = [];
  for (let c = 0; c < fleet.cols; c++) {
    for (let r = 0; r < fleet.rows; r++) {
      if (fleet.alive[r][c]) {
        cols.push(c);
        break;
      }
    }
  }
  return cols;
}

/** Min/max alive column. Returns null when the fleet is empty. */
export function aliveColRange(fleet: Fleet): { min: number; max: number } | null {
  const cols = aliveColumns(fleet);
  if (cols.length === 0) return null;
  return { min: cols[0], max: cols[cols.length - 1] };
}

/** Bottom-most (largest row index) alive alien in a column, or -1. */
export function bottomMostAliveRow(fleet: Fleet, col: number): number {
  for (let r = fleet.rows - 1; r >= 0; r--) if (fleet.alive[r][col]) return r;
  return -1;
}

/** Bottom edge of the lowest alive alien. */
export function fleetBottomY(fleet: Fleet): number {
  let bottom = -Infinity;
  for (let r = 0; r < fleet.rows; r++) {
    for (let c = 0; c < fleet.cols; c++) {
      if (fleet.alive[r][c]) {
        const y = alienRect(fleet, r, c).y + fleet.cellSize.y;
        if (y > bottom) bottom = y;
      }
    }
  }
  return bottom;
}

/** March speed in px/sec. The fewer aliens left, the faster they go. */
export function marchSpeed(fleet: Fleet, config: GameConfig): number {
  const total = fleet.cols * fleet.rows;
  const alive = aliveCount(fleet);
  const attrition = 1 + ((total - alive) / total) * config.fleet.attritionFactor;
  return Math.min(fleet.speed * attrition, config.fleet.maxSpeed);
}

/**
 * Move the fleet sideways. When the outermost living column reaches a wall the
 * fleet turns around, drops a row and speeds up.
 */
export function updateFleet(
  fleet: Fleet,
  dt: number,
  config: GameConfig,
  events: GameEvent[],
): void {
  const range = aliveColRange(fleet);
  if (!range) return;

  const stride = colStride(fleet);
  const speed = marchSpeed(fleet, config);
  const dx = fleet.dir * speed * dt;

  const left = config.bounds.x;
  const right = config.bounds.x + config.bounds.w;

  const leftEdge = fleet.origin.x + range.min * stride;
  const rightEdge = fleet.origin.x + range.max * stride + fleet.cellSize.x;

  let bounce = false;
  if (fleet.dir > 0 && rightEdge + dx > right) {
    fleet.origin.x = right - (range.max * stride + fleet.cellSize.x);
    bounce = true;
  } else if (fleet.dir < 0 && leftEdge + dx < left) {
    fleet.origin.x = left - range.min * stride;
    bounce = true;
  } else {
    fleet.origin.x += dx;
  }

  if (bounce) {
    fleet.dir = (fleet.dir === 1 ? -1 : 1) as 1 | -1;
    fleet.origin.y += fleet.stepDown;
    fleet.speed = Math.min(
      fleet.speed * config.fleet.descendSpeedup,
      config.fleet.maxSpeed,
    );
    events.push({ type: 'fleetDescended', newSpeed: marchSpeed(fleet, config) });
  }
}
