import DOMPurify from 'dompurify';
import { parseEpub, Book, TocNode, EpubParseError } from '../parser/index.js';
import {
  Navigator,
  Paginator,
  ProgressTracker,
  SettingsStore,
  BookmarkStore,
  type ReaderSettings,
  type Storage as ReaderStorage,
} from '../reader/index.js';

/** Thin localStorage adapter satisfying the reader `Storage` interface. */
const browserStorage: ReaderStorage = {
  get: (k) => localStorage.getItem(k),
  set: (k, v) => localStorage.setItem(k, v),
  remove: (k) => localStorage.removeItem(k),
};

function $(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (!el) throw new Error(`Missing element #${id}`);
  return el;
}

class ReaderApp {
  private book?: Book;
  private nav?: Navigator;
  private progress?: ProgressTracker;
  private bookmarks?: BookmarkStore;
  private readonly paginator = new Paginator();
  private readonly settings = new SettingsStore(browserStorage);
  private readonly blobUrls = new Map<string, string>();
  private currentSpineIndex = 0;

  private readonly app = $('app');
  private readonly content = $('content') as HTMLDivElement;
  private readonly viewport = $('viewport') as HTMLDivElement;

  constructor() {
    this.applySettings(this.settings.get());
    this.wireEvents();
  }

  private wireEvents(): void {
    ($('file-input') as HTMLInputElement).addEventListener('change', (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file) void this.openFile(file);
    });
    $('sample-btn').addEventListener('click', () => void this.openSample());
    $('prev-btn').addEventListener('click', () => this.pageBackward());
    $('next-btn').addEventListener('click', () => this.pageForward());
    $('toc-toggle').addEventListener('click', () => this.toggleToc());
    $('theme-toggle').addEventListener('click', () => this.toggleTheme());
    $('bookmark-btn').addEventListener('click', () => this.addBookmark());

    window.addEventListener('keydown', (e) => this.onKey(e));
    new ResizeObserver(() => this.relayout()).observe(this.viewport);
  }

  private async openFile(file: File): Promise<void> {
    const buf = new Uint8Array(await file.arrayBuffer());
    await this.load(buf);
  }

  private async openSample(): Promise<void> {
    const res = await fetch('/sample.epub');
    if (!res.ok) {
      alert('Sample EPUB not found.');
      return;
    }
    await this.load(new Uint8Array(await res.arrayBuffer()));
  }

  private async load(bytes: Uint8Array): Promise<void> {
    try {
      this.revokeBlobs();
      const book = await parseEpub(bytes);
      this.book = book;
      this.nav = new Navigator(book);
      this.progress = new ProgressTracker(book);
      const bookId = book.metadata.identifier || book.metadata.title || 'book';
      this.bookmarks = new BookmarkStore(browserStorage, bookId);

      $('book-title').textContent = book.metadata.title;
      $('book-author').textContent = book.metadata.authors.length
        ? book.metadata.authors.join(', ')
        : 'Unknown author';
      ($('placeholder') as HTMLElement).style.display = 'none';

      this.renderToc(book.toc);
      this.showChapter(0);
    } catch (err) {
      const msg =
        err instanceof EpubParseError
          ? `Could not open EPUB (${err.code}): ${err.message}`
          : `Could not open EPUB: ${(err as Error).message}`;
      alert(msg);
    }
  }

  private renderToc(nodes: TocNode[]): void {
    const list = $('toc-list');
    list.replaceChildren();
    list.appendChild(this.buildTocList(nodes));
  }

  private buildTocList(nodes: TocNode[]): HTMLUListElement {
    const ul = document.createElement('ul');
    ul.className = 'toc-list';
    for (const node of nodes) {
      const li = document.createElement('li');
      const a = document.createElement('a');
      a.textContent = node.label || '(untitled)';
      if (node.href) {
        a.href = '#';
        a.dataset['href'] = node.href;
        a.addEventListener('click', (e) => {
          e.preventDefault();
          this.goToHref(node.href!);
        });
      }
      li.appendChild(a);
      if (node.children.length) li.appendChild(this.buildTocList(node.children));
      ul.appendChild(li);
    }
    return ul;
  }

  private goToHref(href: string): void {
    if (!this.nav) return;
    try {
      const { spineIndex, fragment } = this.nav.goToHref(href);
      this.showChapter(spineIndex, fragment);
    } catch {
      /* href not in spine; ignore */
    }
  }

  private showChapter(spineIndex: number, fragment?: string): void {
    if (!this.book) return;
    this.currentSpineIndex = spineIndex;
    const fromFile = this.book.spine[spineIndex]!.item.href;
    const { html } = this.book.getChapter(spineIndex);

    const doc = new DOMParser().parseFromString(html, 'text/html');
    this.rewriteResources(doc, fromFile);

    // DOMPurify strips <script>, on* handlers and unsafe URLs by default; we
    // keep <style> so the inlined author CSS survives, and allow epub:type.
    const clean = DOMPurify.sanitize(doc.body.innerHTML, {
      ADD_ATTR: ['epub:type'],
      ADD_TAGS: ['style'],
    });
    this.content.innerHTML = clean;

    this.relayout();
    if (fragment) this.scrollToFragment(fragment);
    this.highlightToc(fromFile);
    this.updateProgress();
  }

  private rewriteResources(doc: Document, fromFile: string): void {
    if (!this.book) return;
    // Images.
    doc.querySelectorAll('img').forEach((img) => {
      const src = img.getAttribute('src');
      if (src) img.setAttribute('src', this.blobFor(fromFile, src) ?? src);
    });
    doc.querySelectorAll('image').forEach((img) => {
      const href =
        img.getAttribute('xlink:href') ?? img.getAttribute('href');
      if (href) {
        const url = this.blobFor(fromFile, href);
        if (url) {
          img.setAttribute('href', url);
          img.setAttribute('xlink:href', url);
        }
      }
    });
    // Stylesheets -> inline <style> (so author CSS layers under user vars).
    doc.querySelectorAll('link[rel="stylesheet"]').forEach((link) => {
      const href = link.getAttribute('href');
      if (!href) return;
      const css = this.cssText(fromFile, href);
      if (css != null) {
        const style = doc.createElement('style');
        style.textContent = css;
        link.replaceWith(style);
      } else {
        link.remove();
      }
    });
  }

  private cssText(fromFile: string, href: string): string | null {
    if (!this.book) return null;
    try {
      const path = this.book.resolveHref(fromFile, href);
      const bytes = this.book.resources.get(path);
      if (!bytes) return null;
      const text = new TextDecoder().decode(bytes);
      // Rewrite url(...) references relative to the CSS file's directory.
      return text.replace(/url\(\s*(['"]?)([^'")]+)\1\s*\)/g, (m, _q, ref) => {
        if (/^(data:|https?:|blob:)/i.test(ref)) return m;
        const url = this.blobFor(path, ref);
        return url ? `url("${url}")` : m;
      });
    } catch {
      return null;
    }
  }

  private blobFor(fromFile: string, href: string): string | null {
    if (!this.book) return null;
    if (/^(data:|https?:|blob:)/i.test(href)) return href;
    let path: string;
    try {
      path = this.book.resolveHref(fromFile, href);
    } catch {
      return null;
    }
    const cached = this.blobUrls.get(path);
    if (cached) return cached;
    const bytes = this.book.resources.get(path);
    if (!bytes) return null;
    const mediaType =
      [...this.book.manifest.values()].find((i) => i.href === path)?.mediaType ??
      'application/octet-stream';
    // Copy into a fresh ArrayBuffer-backed view for a well-typed BlobPart.
    const part = new Uint8Array(bytes);
    const url = URL.createObjectURL(new Blob([part], { type: mediaType }));
    this.blobUrls.set(path, url);
    return url;
  }

  private relayout(): void {
    const vw = this.viewport.clientWidth;
    // Author CSS columns: one column per viewport width.
    this.content.style.columnWidth = `${vw}px`;
    this.content.style.height = `${this.viewport.clientHeight}px`;
    // Let layout settle, then measure.
    const scrollWidth = this.content.scrollWidth;
    this.paginator.measure(scrollWidth, vw);
    this.applyPage();
  }

  private applyPage(): void {
    const offset = this.paginator.currentOffset();
    this.content.style.transform = `translateX(${-offset}px)`;
    this.updateProgress();
  }

  private pageForward(): void {
    if (this.paginator.nextPage() === 'overflow') {
      const item = this.nav?.next();
      if (item) {
        this.showChapter(this.nav!.currentIndex());
        this.paginator.toFirstPage();
        this.applyPage();
      }
    } else {
      this.applyPage();
    }
  }

  private pageBackward(): void {
    if (this.paginator.prevPage() === 'underflow') {
      const item = this.nav?.prev();
      if (item) {
        this.showChapter(this.nav!.currentIndex());
        this.paginator.toLastPage();
        this.applyPage();
      }
    } else {
      this.applyPage();
    }
  }

  private scrollToFragment(fragment: string): void {
    const el = this.content.querySelector(`#${cssEscape(fragment)}`);
    if (!(el instanceof HTMLElement)) return;
    const page = this.paginator.pageForOffset(el.offsetLeft);
    this.paginator.goToPage(page);
    this.applyPage();
  }

  private updateProgress(): void {
    if (!this.progress) return;
    this.progress.setLocation({
      spineIndex: this.currentSpineIndex,
      fraction: this.paginator.pageFraction(),
    });
    const pct = this.progress.percentComplete();
    ($('progress-fill') as HTMLElement).style.width = `${pct}%`;
    $('progress-label').textContent =
      `${Math.round(pct)}% · p${this.paginator.currentPage() + 1}/${this.paginator.pageCount()}`;
  }

  private addBookmark(): void {
    if (!this.bookmarks) return;
    this.bookmarks.add({
      spineIndex: this.currentSpineIndex,
      anchor: String(this.paginator.currentPage()),
      label: $('book-title').textContent ?? undefined,
    });
    const btn = $('bookmark-btn');
    btn.textContent = '★ Bookmarked';
    setTimeout(() => (btn.textContent = '☆ Bookmark'), 1200);
  }

  private highlightToc(fromFile: string): void {
    $('toc-list')
      .querySelectorAll('a[data-href]')
      .forEach((a) => {
        const href = (a as HTMLAnchorElement).dataset['href'] ?? '';
        const path = href.split('#')[0];
        a.setAttribute('aria-current', path === fromFile ? 'true' : 'false');
      });
  }

  private applySettings(s: ReaderSettings): void {
    this.app.setAttribute('data-theme', s.theme);
    const root = this.app.style;
    root.setProperty('--user-font-family', s.fontFamily);
    root.setProperty('--user-font-size', `${s.fontSizePx}px`);
    root.setProperty('--user-line-height', String(s.lineHeight));
  }

  private toggleTheme(): void {
    const next: ReaderSettings = this.settings.save({
      theme: this.settings.get().theme === 'light' ? 'dark' : 'light',
    });
    this.applySettings(next);
    this.relayout();
  }

  private toggleToc(): void {
    const panel = $('toc-panel');
    const hidden = panel.hasAttribute('hidden');
    if (hidden) panel.removeAttribute('hidden');
    else panel.setAttribute('hidden', '');
    $('toc-toggle').setAttribute('aria-expanded', String(hidden));
    this.relayout();
  }

  private onKey(e: KeyboardEvent): void {
    if (e.key === 'ArrowRight' || e.key === 'PageDown') this.pageForward();
    else if (e.key === 'ArrowLeft' || e.key === 'PageUp') this.pageBackward();
    else if (e.key === 't' || e.key === 'T') this.toggleToc();
    else if (e.key === 'b' || e.key === 'B') this.addBookmark();
    else return;
    e.preventDefault();
  }

  private revokeBlobs(): void {
    for (const url of this.blobUrls.values()) URL.revokeObjectURL(url);
    this.blobUrls.clear();
  }
}

function cssEscape(s: string): string {
  const cssApi = (window as unknown as { CSS?: { escape?: (v: string) => string } }).CSS;
  return cssApi?.escape ? cssApi.escape(s) : s.replace(/[^a-zA-Z0-9_-]/g, '\\$&');
}

new ReaderApp();
