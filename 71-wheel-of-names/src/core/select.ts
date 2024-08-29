import type { Segment } from "./geometry.js";
import type { RNG } from "./rng.js";
import { EmptyWheelError } from "./errors.js";

/** Pick a segment with probability proportional to its arc (and so its weight). */
export function pickWinner(segs: readonly Segment[], rng: RNG): number {
  if (segs.length === 0) throw new EmptyWheelError();
  if (segs.length === 1) return segs[0].index;

  const total = segs.reduce((s, seg) => s + (seg.endAngle - seg.startAngle), 0);
  const target = rng.next() * total;

  let acc = 0;
  for (const seg of segs) {
    acc += seg.endAngle - seg.startAngle;
    if (target < acc) return seg.index;
  }
  // Only reachable through rounding.
  return segs[segs.length - 1].index;
}
