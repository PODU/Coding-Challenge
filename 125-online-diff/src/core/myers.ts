/**
 * Myers O(ND) diff over arbitrary sequences (the algorithm Git uses).
 *
 * Returns contiguous segments covering both inputs in order. Within each run
 * of changes, all deletions come before all insertions, which is what line
 * pairing and patch output expect.
 */

export type SegmentType = 'equal' | 'delete' | 'insert';

export interface Segment {
  type: SegmentType;
  /** For inserts, the position in `a` where the insert happens. */
  aStart: number;
  aCount: number;
  /** For deletes, the matching position in `b`. */
  bStart: number;
  bCount: number;
}

export interface DiffSeqOptions<T> {
  eq?: (x: T, y: T) => boolean;
  /**
   * Maximum edit distance to search (default 1000). Myers costs O((N+M)·D)
   * time and memory, and for very different inputs a minimal diff isn't worth
   * it. Past the cap, the middle part becomes one delete block plus one
   * insert block.
   */
  maxD?: number;
}

const DEFAULT_MAX_D = 1000;

export function diffSequences<T>(
  a: readonly T[],
  b: readonly T[],
  opts: DiffSeqOptions<T> = {},
): Segment[] {
  const eq = opts.eq ?? ((x: T, y: T) => x === y);
  const maxD = opts.maxD ?? DEFAULT_MAX_D;

  // Trimming the common prefix and suffix is cheap and shrinks the trace.
  let pre = 0;
  while (pre < a.length && pre < b.length && eq(a[pre], b[pre])) pre++;
  let suf = 0;
  while (
    suf < a.length - pre &&
    suf < b.length - pre &&
    eq(a[a.length - 1 - suf], b[b.length - 1 - suf])
  ) {
    suf++;
  }

  const aMid = a.slice(pre, a.length - suf);
  const bMid = b.slice(pre, b.length - suf);
  const ops = myersOps(aMid, bMid, eq, maxD);

  const segs: Segment[] = [];
  let aPos = 0;
  let bPos = 0;
  const emit = (type: SegmentType, aCount: number, bCount: number): void => {
    if (aCount === 0 && bCount === 0) return;
    const last = segs[segs.length - 1];
    if (last && last.type === type) {
      last.aCount += aCount;
      last.bCount += bCount;
    } else {
      segs.push({ type, aStart: aPos, aCount, bStart: bPos, bCount });
    }
    aPos += aCount;
    bPos += bCount;
  };

  emit('equal', pre, pre);
  for (const op of ops) {
    if (op === 0) emit('equal', 1, 1);
    else if (op === -1) emit('delete', 1, 0);
    else emit('insert', 0, 1);
  }
  emit('equal', suf, suf);

  return coalesceChangeBlocks(segs);
}

/**
 * Myers can interleave deletes and inserts; collapse each run of changes into
 * one delete followed by one insert.
 */
function coalesceChangeBlocks(segs: Segment[]): Segment[] {
  const out: Segment[] = [];
  let i = 0;
  while (i < segs.length) {
    if (segs[i].type === 'equal') {
      out.push(segs[i]);
      i++;
      continue;
    }
    const aStart = segs[i].aStart;
    const bStart = segs[i].bStart;
    let aCount = 0;
    let bCount = 0;
    while (i < segs.length && segs[i].type !== 'equal') {
      aCount += segs[i].aCount;
      bCount += segs[i].bCount;
      i++;
    }
    if (aCount > 0) out.push({ type: 'delete', aStart, aCount, bStart, bCount: 0 });
    if (bCount > 0) {
      out.push({ type: 'insert', aStart: aStart + aCount, aCount: 0, bStart, bCount });
    }
  }
  return out;
}

/**
 * Greedy Myers search, keeping every V array for the backtrack. Returns one op
 * per step: 0 = equal, -1 = delete from a, 1 = insert from b.
 */
function myersOps<T>(
  a: readonly T[],
  b: readonly T[],
  eq: (x: T, y: T) => boolean,
  maxD: number,
): number[] {
  const n = a.length;
  const m = b.length;
  if (n === 0) return new Array<number>(m).fill(1);
  if (m === 0) return new Array<number>(n).fill(-1);

  const limit = Math.min(n + m, maxD);
  const off = limit;
  // k stays within [-limit, limit], and the k === -d / k === d branches never
  // read outside the band, so v[off + k ± 1] is always in bounds.
  const v = new Int32Array(2 * limit + 1);
  const trace: Int32Array[] = [];
  let dFinal = -1;

  outer: for (let d = 0; d <= limit; d++) {
    trace.push(v.slice()); // V as it was after depth d-1
    for (let k = -d; k <= d; k += 2) {
      let x: number;
      if (k === -d || (k !== d && v[off + k - 1] < v[off + k + 1])) {
        x = v[off + k + 1]; // move down (insert)
      } else {
        x = v[off + k - 1] + 1; // move right (delete)
      }
      let y = x - k;
      while (x < n && y < m && eq(a[x], b[y])) {
        x++;
        y++;
      }
      v[off + k] = x;
      if (x >= n && y >= m) {
        dFinal = d;
        break outer;
      }
    }
  }

  if (dFinal < 0) {
    // Hit the cap: replace everything. Correct, just not minimal.
    const ops = new Array<number>(n).fill(-1);
    for (let j = 0; j < m; j++) ops.push(1);
    return ops;
  }

  const ops: number[] = [];
  let x = n;
  let y = m;
  for (let d = dFinal; d > 0; d--) {
    const vp = trace[d];
    const k = x - y;
    const prevK =
      k === -d || (k !== d && vp[off + k - 1] < vp[off + k + 1]) ? k + 1 : k - 1;
    const prevX = vp[off + prevK];
    const prevY = prevX - prevK;
    while (x > prevX && y > prevY) {
      ops.push(0);
      x--;
      y--;
    }
    if (prevK === k + 1) {
      ops.push(1); // came from above: insert b[y-1]
      y--;
    } else {
      ops.push(-1); // came from the left: delete a[x-1]
      x--;
    }
  }
  while (x > 0 && y > 0) {
    ops.push(0); // the d = 0 diagonal
    x--;
    y--;
  }
  ops.reverse();
  return ops;
}
