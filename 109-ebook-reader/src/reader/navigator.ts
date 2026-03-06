import { Book, EpubParseError, SpineItem } from '../parser/index.js';
import { PathResolver } from '../parser/index.js';

export interface HrefTarget {
  spineIndex: number;
  fragment?: string;
}

/**
 * Owns a cursor over the spine. `next`/`prev` walk the primary (linear) flow,
 * skipping `linear="no"` items; `goToSpineIndex` can target any item.
 */
export class Navigator {
  private readonly book: Book;
  private cursor: number;

  constructor(book: Book, startIndex = 0) {
    this.book = book;
    if (book.spine.length === 0) {
      throw new EpubParseError('Book has an empty spine', 'EMPTY_SPINE');
    }
    this.cursor = clamp(startIndex, 0, book.spine.length - 1);
  }

  current(): SpineItem {
    return this.book.spine[this.cursor]!;
  }

  currentIndex(): number {
    return this.cursor;
  }

  /** Advance to the next linear item, or null if already at/after the last. */
  next(): SpineItem | null {
    for (let i = this.cursor + 1; i < this.book.spine.length; i++) {
      if (this.book.spine[i]!.linear) {
        this.cursor = i;
        return this.current();
      }
    }
    return null;
  }

  /** Step back to the previous linear item, or null if at/before the first. */
  prev(): SpineItem | null {
    for (let i = this.cursor - 1; i >= 0; i--) {
      if (this.book.spine[i]!.linear) {
        this.cursor = i;
        return this.current();
      }
    }
    return null;
  }

  goToSpineIndex(i: number): SpineItem {
    if (i < 0 || i >= this.book.spine.length) {
      throw new EpubParseError(`Spine index out of range: ${i}`, 'BAD_SPINE_INDEX');
    }
    this.cursor = i;
    return this.current();
  }

  /**
   * Resolve a TOC/anchor href (already archive-relative, possibly with a
   * `#fragment`) to a spine index. Also moves the cursor there.
   */
  goToHref(href: string): HrefTarget {
    const { path, fragment } = PathResolver.splitFragment(href);
    // The href may itself be relative; normalize defensively.
    let target = path;
    try {
      target = PathResolver.resolve('', path);
    } catch {
      /* use raw path */
    }
    const idx = this.book.spine.findIndex((s) => s.item.href === target);
    if (idx < 0) {
      throw new EpubParseError(`No spine item for href: ${href}`, 'HREF_NOT_FOUND');
    }
    this.cursor = idx;
    const result: HrefTarget = { spineIndex: idx };
    if (fragment) result.fragment = fragment;
    return result;
  }

  /** Position over the full spine (linear + non-linear). */
  position(): { spineIndex: number; total: number } {
    return { spineIndex: this.cursor, total: this.book.spine.length };
  }
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}
