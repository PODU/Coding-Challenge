import {
  Book,
  ChapterDoc,
  EpubParseError,
  SpineItem,
  TocNode,
} from './types.js';
import { OcfReader } from './ocfReader.js';
import { ContainerXml } from './containerXml.js';
import { OpfParser } from './opfParser.js';
import { TocParser } from './tocParser.js';
import { PathResolver } from './pathResolver.js';

const decoder = new TextDecoder('utf-8');

/**
 * Parse EPUB bytes into a {@link Book}. Uses no DOM APIs.
 *
 * @throws {EpubParseError} on fatal problems (bad zip, missing container, DRM).
 */
export async function parseEpub(bytes: Uint8Array): Promise<Book> {
  const warnings: string[] = [];

  const resources = OcfReader.open(bytes);
  const mimeWarning = OcfReader.checkMimetype(resources);
  if (mimeWarning) warnings.push(mimeWarning);

  const { opfPath, warnings: containerWarnings } =
    ContainerXml.opfPath(resources);
  warnings.push(...containerWarnings);

  const opfBytes = resources.get(opfPath);
  if (!opfBytes) {
    throw new EpubParseError(
      `OPF package not found at ${opfPath}`,
      'NO_OPF',
    );
  }
  const opfBase = PathResolver.dirname(opfPath);

  const opf = OpfParser.parse(opfBytes, opfBase);
  warnings.push(...opf.warnings);

  // Prefer the nav document, then the NCX, else build a TOC from the spine.
  const toc = buildToc(opf.navHref, opf.ncxHref, opf.spine, resources, warnings);

  const book: Book = {
    version: opf.version,
    metadata: opf.metadata,
    manifest: opf.manifest,
    spine: opf.spine,
    toc,
    resources,
    warnings,
    getChapter(spineIndex: number): ChapterDoc {
      const entry = opf.spine[spineIndex];
      if (!entry) {
        throw new EpubParseError(
          `No spine item at index ${spineIndex}`,
          'BAD_SPINE_INDEX',
        );
      }
      const data = resources.get(entry.item.href);
      if (!data) {
        throw new EpubParseError(
          `Chapter resource missing: ${entry.item.href}`,
          'MISSING_RESOURCE',
        );
      }
      return {
        html: decoder.decode(data),
        basePath: PathResolver.dirname(entry.item.href),
      };
    },
    resolveHref(fromDoc: string, href: string): string {
      return PathResolver.resolve(PathResolver.dirname(fromDoc), href);
    },
  };
  if (opf.coverHref) book.coverHref = opf.coverHref;
  return book;
}

function buildToc(
  navHref: string | undefined,
  ncxHref: string | undefined,
  spine: SpineItem[],
  resources: Map<string, Uint8Array>,
  warnings: string[],
): TocNode[] {
  // EPUB 3 nav document (preferred).
  if (navHref) {
    const navBytes = resources.get(navHref);
    if (navBytes) {
      try {
        const nodes = TocParser.fromNav(navBytes, PathResolver.dirname(navHref));
        if (nodes.length) return nodes;
        warnings.push('Nav document produced an empty TOC; falling back');
      } catch (err) {
        warnings.push(`Nav document malformed (${(err as Error).message})`);
      }
    }
  }

  // EPUB 2 NCX fallback.
  if (ncxHref) {
    const ncxBytes = resources.get(ncxHref);
    if (ncxBytes) {
      try {
        const nodes = TocParser.fromNcx(ncxBytes, PathResolver.dirname(ncxHref));
        if (nodes.length) return nodes;
        warnings.push('NCX produced an empty TOC; synthesizing from spine');
      } catch (err) {
        warnings.push(`NCX malformed (${(err as Error).message})`);
      }
    }
  }

  // Synthesize a flat TOC from the linear spine.
  warnings.push('No usable TOC found; synthesized from spine');
  return TocParser.synthesize(spine, (item) =>
    chapterLabel(resources.get(item.item.href), item.item.href),
  );
}

/** Chapter label from its `<title>`, first heading, or file name. */
function chapterLabel(data: Uint8Array | undefined, href: string): string {
  const fallback = decodeURIComponent(href.split('/').pop() ?? href);
  if (!data) return fallback;
  const html = decoder.decode(data);
  const title = /<title[^>]*>([\s\S]*?)<\/title>/i.exec(html)?.[1];
  if (title && title.trim()) return collapse(title);
  const heading = /<h[1-6][^>]*>([\s\S]*?)<\/h[1-6]>/i.exec(html)?.[1];
  if (heading && heading.trim()) return collapse(stripTags(heading));
  return fallback;
}

function stripTags(s: string): string {
  return s.replace(/<[^>]+>/g, ' ');
}
function collapse(s: string): string {
  return s.replace(/\s+/g, ' ').trim();
}
