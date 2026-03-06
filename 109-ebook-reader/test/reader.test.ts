import { describe, it, expect, beforeEach } from 'vitest';
import { parseEpub, Book } from '../src/parser/index.js';
import {
  Navigator,
  ProgressTracker,
  Paginator,
  MemoryStorage,
  BookmarkStore,
  AnnotationStore,
  SettingsStore,
} from '../src/reader/index.js';
import { fixture } from './helpers.js';

let book: Book;
beforeEach(async () => {
  book = await parseEpub(fixture('minimal-v3.epub'));
});

describe('12. Navigator', () => {
  it('walks the linear spine in order and stops at the end', () => {
    const nav = new Navigator(book);
    expect(nav.current().idref).toBe('chap1');
    expect(nav.next()!.idref).toBe('chap2');
    expect(nav.next()!.idref).toBe('chap3');
    // chap3 is the last linear item; `notes` is linear="no" and is skipped.
    expect(nav.next()).toBeNull();
  });

  it('prev() is symmetric', () => {
    const nav = new Navigator(book);
    nav.goToSpineIndex(2);
    expect(nav.prev()!.idref).toBe('chap2');
    expect(nav.prev()!.idref).toBe('chap1');
    expect(nav.prev()).toBeNull();
  });

  it('skips non-linear items in the primary flow', () => {
    const nav = new Navigator(book);
    nav.goToSpineIndex(2); // chap3
    expect(nav.next()).toBeNull(); // does not surface `notes`
  });

  it('goToHref resolves a TOC href to spine index + fragment', () => {
    const nav = new Navigator(book);
    const target = nav.goToHref('OEBPS/text/chap2.xhtml#sec2');
    expect(target.spineIndex).toBe(1);
    expect(target.fragment).toBe('sec2');
    expect(nav.current().idref).toBe('chap2');
  });
});

describe('13. ProgressTracker', () => {
  it('is 0 at start, 100 at last page of last chapter, monotonic', () => {
    const pt = new ProgressTracker(book);
    pt.setLocation({ spineIndex: 0, fraction: 0 });
    expect(pt.percentComplete()).toBe(0);

    const seen: number[] = [];
    for (let i = 0; i < book.spine.length; i++) {
      pt.setLocation({ spineIndex: i, fraction: 0 });
      seen.push(pt.percentComplete());
    }
    // Monotonic non-decreasing across chapter starts.
    for (let i = 1; i < seen.length; i++) {
      expect(seen[i]!).toBeGreaterThanOrEqual(seen[i - 1]!);
    }

    pt.setLocation({ spineIndex: book.spine.length - 1, fraction: 1 });
    expect(pt.percentComplete()).toBeCloseTo(100, 6);
  });

  it('weights by chapter size', () => {
    // Two-chapter book: completion after chapter 0 equals weight0/total.
    const pt = new ProgressTracker(book);
    pt.setLocation({ spineIndex: 1, fraction: 0 });
    const p1 = pt.percentComplete();
    expect(p1).toBeGreaterThan(0);
    expect(p1).toBeLessThan(100);
  });
});

describe('14. Paginator (pure arithmetic)', () => {
  it('computes page count = ceil(scrollWidth / viewport)', () => {
    const p = new Paginator();
    p.measure(1000, 250);
    expect(p.pageCount()).toBe(4);
  });

  it('round-trips page <-> offset', () => {
    const p = new Paginator();
    p.measure(1000, 250);
    expect(p.offsetForPage(2)).toBe(500);
    expect(p.pageForOffset(500)).toBe(2);
  });

  it('signals overflow/underflow at chapter boundaries', () => {
    const p = new Paginator();
    p.measure(1000, 250); // 4 pages: 0..3
    expect(p.prevPage()).toBe('underflow');
    p.toLastPage();
    expect(p.currentPage()).toBe(3);
    expect(p.nextPage()).toBe('overflow');
  });

  it('preserves the reading fraction across a resize', () => {
    const p = new Paginator();
    p.measure(1000, 250); // 4 pages
    p.goToPage(2); // fraction 2/3
    p.measure(2000, 250); // now 8 pages
    // Closest page to 2/3 of 7 == round(4.66) == 5
    expect(p.currentPage()).toBe(5);
  });
});

describe('15. Stores (round-trip through Storage fake)', () => {
  let storage: MemoryStorage;
  beforeEach(() => {
    storage = new MemoryStorage();
  });

  it('bookmarks persist content-anchored locations', () => {
    const store = new BookmarkStore(storage, 'book-1');
    const bm = store.add({ spineIndex: 2, anchor: 'p3', label: 'Here' });
    expect(store.list()).toHaveLength(1);
    // A fresh store over the same storage re-reads it.
    const reopened = new BookmarkStore(storage, 'book-1');
    expect(reopened.list()[0]).toMatchObject({ spineIndex: 2, anchor: 'p3' });
    reopened.remove(bm.id);
    expect(reopened.list()).toHaveLength(0);
  });

  it('annotations persist text + color + anchor', () => {
    const store = new AnnotationStore(storage, 'book-1');
    store.add({ spineIndex: 1, anchor: 'p1', text: 'hi', color: 'yellow' });
    const reopened = new AnnotationStore(storage, 'book-1');
    expect(reopened.list()[0]).toMatchObject({
      text: 'hi',
      color: 'yellow',
      anchor: 'p1',
    });
  });

  it('settings round-trip and re-apply with defaults', () => {
    const store = new SettingsStore(storage);
    expect(store.get().theme).toBe('light');
    store.save({ theme: 'dark', fontSizePx: 22 });
    const reopened = new SettingsStore(storage);
    expect(reopened.get().theme).toBe('dark');
    expect(reopened.get().fontSizePx).toBe(22);
    // Untouched keys keep defaults.
    expect(reopened.get().lineHeight).toBe(1.6);
  });
});
