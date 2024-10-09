/**
 * Wrap-aware circle overlap tests.
 */

import type { Vec2 } from "./vec2.js";

/**
 * Squared distance between two points on a toroidal field, taking the shorter
 * of the direct and across-the-seam delta on each axis.
 */
export function wrappedDist2(a: Vec2, b: Vec2, w: number, h: number): number {
  let dx = Math.abs(a.x - b.x);
  let dy = Math.abs(a.y - b.y);
  if (dx > w - dx) dx = w - dx;
  if (dy > h - dy) dy = h - dy;
  return dx * dx + dy * dy;
}

/** True when two circles overlap, accounting for screen wrap. */
export function circlesOverlap(
  a: Vec2,
  ra: number,
  b: Vec2,
  rb: number,
  w: number,
  h: number,
): boolean {
  const r = ra + rb;
  return wrappedDist2(a, b, w, h) < r * r;
}
