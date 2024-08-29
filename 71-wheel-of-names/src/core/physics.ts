import { type Segment, angleToIndex, normalize, TAU } from "./geometry.js";
import type { RNG } from "./rng.js";
import { pickWinner } from "./select.js";
import { EmptyWheelError } from "./errors.js";

export interface SpinPlan {
  /** Absolute wheel angle (radians) to animate to. */
  targetAngle: number;
  /** Animation duration in milliseconds. */
  durationMs: number;
  /** The index the pointer will land on at targetAngle. */
  winnerIndex: number;
}

export interface SpinOptions {
  minTurns?: number;
  maxTurns?: number;
  durationMs?: number;
  /** Fractional margin kept away from each arc edge, in (0, 0.5). */
  edgeMargin?: number;
}

const DEFAULTS = {
  minTurns: 4,
  maxTurns: 8,
  durationMs: 5000,
  edgeMargin: 0.08,
};

/** Cubic ease-out, mapping [0,1] to [0,1]. */
export function easeOutCubic(t: number): number {
  const u = 1 - t;
  return 1 - u * u * u;
}

/**
 * The pointer is at angle 0 (3 o'clock), so with the wheel turned by
 * `wheelAngle` the pointer is over local angle -wheelAngle. planSpin and
 * resolveWinner both go through here so they can't disagree.
 */
function localAngleUnderPointer(wheelAngle: number): number {
  return normalize(-wheelAngle);
}

/** The segment under the pointer when the wheel stops at finalAngle. */
export function resolveWinner(segs: readonly Segment[], finalAngle: number): number {
  if (segs.length === 0) throw new EmptyWheelError();
  return angleToIndex(segs, localAngleUnderPointer(finalAngle));
}

/**
 * Plan a spin: pick the winner, pick a random spot inside its arc (away from
 * the edges), and work out a target angle at least `minTurns` turns ahead of
 * `fromAngle` that stops the pointer on that spot. resolveWinner on the
 * target angle gives back winnerIndex.
 */
export function planSpin(
  segs: readonly Segment[],
  rng: RNG,
  fromAngle: number,
  opts: SpinOptions = {},
): SpinPlan {
  if (segs.length === 0) throw new EmptyWheelError();

  const minTurns = opts.minTurns ?? DEFAULTS.minTurns;
  const maxTurns = Math.max(minTurns, opts.maxTurns ?? DEFAULTS.maxTurns);
  const durationMs = opts.durationMs ?? DEFAULTS.durationMs;
  const edgeMargin = clamp(opts.edgeMargin ?? DEFAULTS.edgeMargin, 1e-4, 0.49);

  const winnerIndex = pickWinner(segs, rng);
  const seg = segs[winnerIndex];
  const arc = seg.endAngle - seg.startAngle;

  const frac = edgeMargin + rng.next() * (1 - 2 * edgeMargin);
  const localTarget = seg.startAngle + frac * arc;

  // targetAngle must equal -localTarget mod TAU. Take the first such angle at
  // least baseTurns turns past fromAngle.
  const baseTurns = minTurns + Math.floor(rng.next() * (maxTurns - minTurns + 1));
  const floor = fromAngle + baseTurns * TAU;
  const m = Math.ceil((floor + localTarget) / TAU);
  const targetAngle = m * TAU - localTarget;

  return { targetAngle, durationMs, winnerIndex };
}

function clamp(x: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, x));
}
