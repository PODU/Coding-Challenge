import { Book } from '../parser/index.js';

export interface ReadingLocation {
  spineIndex: number;
  /** Element id or text path; unlike a page number it survives reflow. */
  anchor?: string;
  /** Fraction read within the current chapter, 0..1 (page-derived by the UI). */
  fraction?: number;
}

/**
 * Tracks a content-anchored reading position and computes overall progress,
 * weighted by chapter byte size so a long chapter counts for more than a short
 * one. Plain arithmetic, no DOM.
 */
export class ProgressTracker {
  private readonly weights: number[];
  private readonly total: number;
  private location: ReadingLocation;

  constructor(book: Book) {
    this.weights = book.spine.map((s) => {
      const bytes = book.resources.get(s.item.href);
      return Math.max(1, bytes ? bytes.length : 1);
    });
    this.total = this.weights.reduce((a, b) => a + b, 0);
    this.location = { spineIndex: 0, fraction: 0 };
  }

  setLocation(loc: ReadingLocation): void {
    this.location = {
      spineIndex: loc.spineIndex,
      ...(loc.anchor !== undefined ? { anchor: loc.anchor } : {}),
      fraction: clamp01(loc.fraction ?? 0),
    };
  }

  getLocation(): ReadingLocation {
    return { ...this.location };
  }

  /** Overall completion 0..100, weighted by chapter size. */
  percentComplete(): number {
    const i = this.location.spineIndex;
    let done = 0;
    for (let k = 0; k < i && k < this.weights.length; k++) done += this.weights[k]!;
    const frac = clamp01(this.location.fraction ?? 0);
    done += frac * (this.weights[i] ?? 0);
    if (this.total === 0) return 0;
    return (done / this.total) * 100;
  }
}

function clamp01(n: number): number {
  if (Number.isNaN(n)) return 0;
  return Math.max(0, Math.min(1, n));
}
