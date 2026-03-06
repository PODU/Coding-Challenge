/**
 * Pure pagination math for CSS multi-column layout. The UI measures the
 * rendered chapter and injects `scrollWidth`/`viewportWidth`; this class holds
 * the page<->offset arithmetic so it is unit-testable without a DOM.
 */
export class Paginator {
  private scrollWidth = 0;
  private viewportWidth = 1;
  private page = 0;

  /** Recompute layout from fresh measurements, preserving the page fraction. */
  measure(scrollWidth: number, viewportWidth: number): void {
    const prevFraction = this.pageFraction();
    this.scrollWidth = Math.max(0, scrollWidth);
    this.viewportWidth = Math.max(1, viewportWidth);
    // Restore the closest page to the previous reading fraction (anchor-like).
    this.page = clamp(Math.round(prevFraction * (this.pageCount() - 1)), 0, this.pageCount() - 1);
  }

  pageCount(): number {
    return Math.max(1, Math.ceil(this.scrollWidth / this.viewportWidth));
  }

  currentPage(): number {
    return this.page;
  }

  offsetForPage(page: number): number {
    return clamp(page, 0, this.pageCount() - 1) * this.viewportWidth;
  }

  pageForOffset(offset: number): number {
    return clamp(Math.round(offset / this.viewportWidth), 0, this.pageCount() - 1);
  }

  currentOffset(): number {
    return this.offsetForPage(this.page);
  }

  /** Fraction through the chapter, 0..1 (used for progress + anchoring). */
  pageFraction(): number {
    const last = this.pageCount() - 1;
    return last <= 0 ? 0 : this.page / last;
  }

  /** Go to a page. Returns `'underflow'`/`'overflow'` when out of range so the
   *  UI can delegate a chapter boundary crossing to the Navigator. */
  goToPage(page: number): 'ok' | 'underflow' | 'overflow' {
    if (page < 0) return 'underflow';
    if (page > this.pageCount() - 1) return 'overflow';
    this.page = page;
    return 'ok';
  }

  nextPage(): 'ok' | 'overflow' {
    return this.goToPage(this.page + 1) === 'ok' ? 'ok' : 'overflow';
  }

  prevPage(): 'ok' | 'underflow' {
    return this.goToPage(this.page - 1) === 'ok' ? 'ok' : 'underflow';
  }

  /** Jump to the first/last page (used when entering a chapter forwards/back). */
  toFirstPage(): void {
    this.page = 0;
  }
  toLastPage(): void {
    this.page = this.pageCount() - 1;
  }
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}
