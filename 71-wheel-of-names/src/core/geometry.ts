import { assignColors } from "./colors.js";

export const TAU = Math.PI * 2;

/** Zero, negative and NaN weights are replaced with this. */
const MIN_WEIGHT = 1e-6;

export interface Segment {
  index: number;
  label: string;
  weight: number;
  /** Inclusive start angle in radians, measured clockwise from the pointer. */
  startAngle: number;
  /** Exclusive end angle in radians (== next segment's startAngle). */
  endAngle: number;
  /** Centre angle of the arc (for label placement). */
  midAngle: number;
  color: string;
}

export interface BuildOptions {
  /** Relative sizes; non-positive / NaN values are clamped to a tiny epsilon. */
  weights?: number[];
  /** Per-segment colours; falls back to assignColors() when absent. */
  colors?: string[];
}

/** Normalise any angle into the half-open range [0, TAU). */
export function normalize(angle: number): number {
  const a = angle % TAU;
  return a < 0 ? a + TAU : a;
}

function sanitizeWeight(w: number | undefined): number {
  if (w === undefined || !Number.isFinite(w) || w <= 0) return MIN_WEIGHT;
  return w;
}

/**
 * Build the segments. Arcs are equal unless `weights` are given, in which case
 * each arc is proportional to its weight. The segments cover [0, TAU) as
 * half-open intervals, so every angle belongs to exactly one.
 */
export function buildSegments(names: readonly string[], opts: BuildOptions = {}): Segment[] {
  const n = names.length;
  if (n === 0) return [];

  const rawWeights = opts.weights;
  const weights: number[] = names.map((_, i) =>
    rawWeights ? sanitizeWeight(rawWeights[i]) : 1,
  );
  const total = weights.reduce((s, w) => s + w, 0);

  const colors = opts.colors && opts.colors.length === n ? opts.colors.slice() : assignColors(n);

  const segs: Segment[] = [];
  let cursor = 0;
  for (let i = 0; i < n; i++) {
    const arc = (weights[i] / total) * TAU;
    const startAngle = cursor;
    // End the last segment exactly at TAU so rounding can't leave a gap.
    const endAngle = i === n - 1 ? TAU : cursor + arc;
    segs.push({
      index: i,
      label: names[i],
      weight: weights[i],
      startAngle,
      endAngle,
      midAngle: (startAngle + endAngle) / 2,
      color: colors[i],
    });
    cursor = endAngle;
  }
  return segs;
}

/**
 * Return the index of the segment containing `angle`, measured in the wheel's
 * own frame. Any angle is accepted; a boundary belongs to the segment that
 * starts there.
 */
export function angleToIndex(segs: readonly Segment[], angle: number): number {
  if (segs.length === 0) {
    throw new Error("angleToIndex: no segments");
  }
  const a = normalize(angle);
  for (const seg of segs) {
    if (a >= seg.startAngle && a < seg.endAngle) {
      return seg.index;
    }
  }
  // Unreachable while the segments cover [0, TAU).
  return segs[segs.length - 1].index;
}
