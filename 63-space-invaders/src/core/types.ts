export interface Vec2 {
  x: number;
  y: number;
}

export interface Rect {
  x: number; // left
  y: number; // top
  w: number;
  h: number;
}

/** Rectangle overlap. Rectangles that only touch don't count. */
export function intersects(a: Rect, b: Rect): boolean {
  return (
    a.x < b.x + b.w &&
    a.x + a.w > b.x &&
    a.y < b.y + b.h &&
    a.y + a.h > b.y
  );
}

export function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}

export type GamePhase = 'ready' | 'playing' | 'waveCleared' | 'gameOver';

export interface Projectile {
  pos: Vec2; // top-left
  vel: Vec2; // px/sec
  w: number;
  h: number;
}

export function projectileRect(p: Projectile): Rect {
  return { x: p.pos.x, y: p.pos.y, w: p.w, h: p.h };
}

export interface Ship {
  pos: Vec2; // top-left
  w: number;
  h: number;
  lives: number;
  fireCooldown: number; // seconds until the ship can fire again
}

export function shipRect(s: Ship): Rect {
  return { x: s.pos.x, y: s.pos.y, w: s.w, h: s.h };
}

export interface Fleet {
  cols: number;
  rows: number;
  alive: boolean[][]; // [row][col]
  origin: Vec2; // top-left of the grid
  cellSize: Vec2;
  gap: Vec2;
  dir: 1 | -1;
  speed: number; // px/sec before the attrition speed-up; grows on each descent
  stepDown: number;
  bombTimer: number;
}

export interface BunkerCellHit {
  bunker: number;
  cell: Vec2; // x is the column, y the row
}

export interface Bunker {
  pos: Vec2; // top-left
  cols: number;
  rows: number;
  cellSize: Vec2;
  cells: boolean[][]; // [row][col], true while intact
}

export interface Ufo {
  pos: Vec2;
  w: number;
  h: number;
  vel: Vec2;
  points: number;
}

export function ufoRect(u: Ufo): Rect {
  return { x: u.pos.x, y: u.pos.y, w: u.w, h: u.h };
}

export interface InputState {
  left: boolean;
  right: boolean;
  fire: boolean;
}

export type GameEvent =
  | { type: 'alienKilled'; row: number; col: number; points: number }
  | { type: 'playerHit'; livesLeft: number }
  | { type: 'bunkerHit'; bunker: number; cell: Vec2 }
  | { type: 'shotFired' }
  | { type: 'bombDropped' }
  | { type: 'fleetDescended'; newSpeed: number }
  | { type: 'ufoSpawned' }
  | { type: 'ufoKilled'; points: number }
  | { type: 'waveCleared'; wave: number }
  | { type: 'gameOver'; score: number };

export interface GameState {
  phase: GamePhase;
  bounds: Rect;
  ship: Ship;
  fleet: Fleet;
  playerShot: Projectile | null; // only one shot can be in flight
  bombs: Projectile[];
  bunkers: Bunker[];
  ufo: Ufo | null;
  score: number;
  wave: number; // 1-based
  rngState: number;
}
