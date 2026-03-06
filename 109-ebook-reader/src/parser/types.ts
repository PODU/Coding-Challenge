/**
 * Types produced by the EPUB parser. This file has no runtime imports so the
 * reader and UI can depend on it freely.
 */

export type EpubVersion = '2.0' | '3.0' | 'unknown';

export interface BookMetadata {
  title: string;
  authors: string[];
  language?: string;
  identifier?: string;
  publisher?: string;
  published?: string;
}

export interface ManifestItem {
  /** Manifest @id. */
  id: string;
  /** Resolved, archive-relative path (URL-decoded, `..`/`.` collapsed). */
  href: string;
  /** MIME media-type, e.g. `application/xhtml+xml`. */
  mediaType: string;
  /** Parsed `properties` tokens, e.g. `['nav']`, `['cover-image']`. */
  properties: string[];
}

export interface SpineItem {
  idref: string;
  /** Resolved manifest entry for this spine reference. */
  item: ManifestItem;
  /** false => non-primary flow (e.g. pop-up footnotes). */
  linear: boolean;
}

export interface TocNode {
  label: string;
  /** Resolved path (+ optional `#fragment`). */
  href?: string;
  children: TocNode[];
}

/** Raw chapter document plus the base dir for resolving its relative URLs. */
export interface ChapterDoc {
  html: string;
  basePath: string;
}

export interface Book {
  version: EpubVersion;
  metadata: BookMetadata;
  manifest: Map<string, ManifestItem>;
  /** Reading order. */
  spine: SpineItem[];
  /** Unified TOC from nav | ncx | synthesized. */
  toc: TocNode[];
  coverHref?: string;
  /** path -> bytes for every archive entry. */
  resources: Map<string, Uint8Array>;
  /** Non-fatal parse issues. */
  warnings: string[];

  /** Raw chapter XHTML + base dir for resolving its relative URLs. */
  getChapter(spineIndex: number): ChapterDoc;
  /** Resolve a relative href found inside `fromDoc` against the archive. */
  resolveHref(fromDoc: string, href: string): string;
}

/** Distinct error type for fatal, unrecoverable parse failures. */
export class EpubParseError extends Error {
  readonly code: string;
  constructor(message: string, code = 'EPUB_PARSE_ERROR') {
    super(message);
    this.name = 'EpubParseError';
    this.code = code;
  }
}
