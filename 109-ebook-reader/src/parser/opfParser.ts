import {
  BookMetadata,
  EpubVersion,
  ManifestItem,
  SpineItem,
} from './types.js';
import { PathResolver } from './pathResolver.js';
import { createXmlParser, getText, toArray } from './xml.js';

const decoder = new TextDecoder('utf-8');

export interface OpfResult {
  version: EpubVersion;
  metadata: BookMetadata;
  manifest: Map<string, ManifestItem>;
  spine: SpineItem[];
  /** Resolved nav-document path (EPUB 3), if any. */
  navHref?: string;
  /** Resolved NCX path (EPUB 2 `spine@toc`), if any. */
  ncxHref?: string;
  coverHref?: string;
  warnings: string[];
}

/** Parses the OPF package document into metadata, manifest and spine. */
export class OpfParser {
  static parse(xml: string | Uint8Array, opfBase: string): OpfResult {
    const text = typeof xml === 'string' ? xml : decoder.decode(xml);
    const parsed = createXmlParser().parse(text) as Record<string, unknown>;
    const pkg = parsed['package'] as Record<string, unknown> | undefined;
    const warnings: string[] = [];

    if (!pkg) {
      return {
        version: 'unknown',
        metadata: { title: 'Untitled', authors: [] },
        manifest: new Map(),
        spine: [],
        warnings: ['OPF: missing <package> root'],
      };
    }

    const version = normalizeVersion(String(pkg['@_version'] ?? ''));
    const md = (pkg['metadata'] as Record<string, unknown>) ?? {};
    const metadata = parseMetadata(md);

    const manifest = new Map<string, ManifestItem>();
    const manifestEl = (pkg['manifest'] as Record<string, unknown>) ?? {};
    for (const raw of toArray(manifestEl['item']) as Array<
      Record<string, string>
    >) {
      const id = raw['@_id'];
      const hrefRaw = raw['@_href'];
      if (!id || !hrefRaw) continue;
      let href: string;
      try {
        href = PathResolver.resolve(opfBase, hrefRaw);
      } catch {
        warnings.push(`Manifest item "${id}" has an invalid href: ${hrefRaw}`);
        continue;
      }
      const properties = (raw['@_properties'] ?? '')
        .split(/\s+/)
        .filter(Boolean);
      manifest.set(id, {
        id,
        href,
        mediaType: raw['@_media-type'] ?? 'application/octet-stream',
        properties,
      });
    }

    const spine: SpineItem[] = [];
    const spineEl = (pkg['spine'] as Record<string, unknown>) ?? {};
    for (const raw of toArray(spineEl['itemref']) as Array<
      Record<string, string>
    >) {
      const idref = raw['@_idref'];
      if (!idref) continue;
      const item = manifest.get(idref);
      if (!item) {
        warnings.push(`Spine idref "${idref}" not found in manifest; skipped`);
        continue;
      }
      spine.push({ idref, item, linear: raw['@_linear'] !== 'no' });
    }

    let navHref: string | undefined;
    for (const item of manifest.values()) {
      if (item.properties.includes('nav')) {
        navHref = item.href;
        break;
      }
    }
    let ncxHref: string | undefined;
    const tocAttr = spineEl['@_toc'] as string | undefined;
    if (tocAttr) ncxHref = manifest.get(tocAttr)?.href;
    if (!ncxHref) {
      for (const item of manifest.values()) {
        if (item.mediaType === 'application/x-dtbncx+xml') {
          ncxHref = item.href;
          break;
        }
      }
    }

    let coverHref: string | undefined;
    for (const item of manifest.values()) {
      if (item.properties.includes('cover-image')) {
        coverHref = item.href;
        break;
      }
    }
    if (!coverHref) {
      const metas = toArray(md['meta']) as Array<Record<string, string>>;
      const coverMeta = metas.find((m) => m['@_name'] === 'cover');
      const coverId = coverMeta?.['@_content'];
      if (coverId) coverHref = manifest.get(coverId)?.href;
    }

    const result: OpfResult = { version, metadata, manifest, spine, warnings };
    if (navHref) result.navHref = navHref;
    if (ncxHref) result.ncxHref = ncxHref;
    if (coverHref) result.coverHref = coverHref;
    return result;
  }
}

function normalizeVersion(v: string): EpubVersion {
  if (v.startsWith('3')) return '3.0';
  if (v.startsWith('2')) return '2.0';
  return 'unknown';
}

function parseMetadata(md: Record<string, unknown>): BookMetadata {
  const title = getText(toArray(md['title'])[0]) || 'Untitled';

  const creators = toArray(md['creator']);
  const authors = creators
    .map((c) => getText(c).trim())
    .filter((s) => s.length > 0);

  const metadata: BookMetadata = { title, authors };
  const language = getText(toArray(md['language'])[0]).trim();
  const identifier = getText(toArray(md['identifier'])[0]).trim();
  const publisher = getText(toArray(md['publisher'])[0]).trim();
  const published = getText(toArray(md['date'])[0]).trim();
  if (language) metadata.language = language;
  if (identifier) metadata.identifier = identifier;
  if (publisher) metadata.publisher = publisher;
  if (published) metadata.published = published;
  return metadata;
}
