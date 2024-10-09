/**
 * Minimal 2D vector math.
 *
 * All functions are pure and allocate fresh vectors (no mutation of inputs),
 * so they are safe to use inside the deterministic core.
 */

export type Vec2 = { x: number; y: number };

export const vec = (x: number, y: number): Vec2 => ({ x, y });

export const add = (a: Vec2, b: Vec2): Vec2 => ({ x: a.x + b.x, y: a.y + b.y });

export const sub = (a: Vec2, b: Vec2): Vec2 => ({ x: a.x - b.x, y: a.y - b.y });

export const scale = (a: Vec2, s: number): Vec2 => ({ x: a.x * s, y: a.y * s });

/** Rotate a vector by `rad` radians (positive = clockwise in a y-down frame). */
export const rotate = (a: Vec2, rad: number): Vec2 => {
  const c = Math.cos(rad);
  const s = Math.sin(rad);
  return { x: a.x * c - a.y * s, y: a.x * s + a.y * c };
};

/** Unit vector pointing along `rad` (0 = +x / right). */
export const fromAngle = (rad: number): Vec2 => ({ x: Math.cos(rad), y: Math.sin(rad) });

export const mag2 = (a: Vec2): number => a.x * a.x + a.y * a.y;

export const mag = (a: Vec2): number => Math.sqrt(mag2(a));

/** Clamp a vector's magnitude to `max` (no-op if already within bounds). */
export const limit = (a: Vec2, max: number): Vec2 => {
  const m2 = mag2(a);
  if (m2 <= max * max || m2 === 0) return { x: a.x, y: a.y };
  const m = Math.sqrt(m2);
  return { x: (a.x / m) * max, y: (a.y / m) * max };
};
