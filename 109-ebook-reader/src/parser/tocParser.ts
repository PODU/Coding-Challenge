import { SpineItem, TocNode } from './types.js';
import { PathResolver } from './pathResolver.js';
import { createXmlParser, getText, toArray } from './xml.js';

const decoder = new TextDecoder('utf-8');

/** Builds a unified {@link TocNode} tree from nav, NCX, or the spine. */
export class TocParser {
  /** EPUB 3: parse the navigation document's `<nav epub:type="toc">`. */
  static fromNav(xml: string | Uint8Array, baseDir: string): TocNode[] {
    const text = typeof xml === 'string' ? xml : decoder.decode(xml);
    const parsed = createXmlParser().parse(text) as Record<string, unknown>;
    const html = (parsed['html'] as Record<string, unknown>) ?? {};
    const body = (html['body'] as Record<string, unknown>) ?? {};
    const navs = toArray(body['nav']) as Array<Record<string, unknown>>;

    const tocNav =
      navs.find((n) => String(n['@_type'] ?? '').includes('toc')) ?? navs[0];
    if (!tocNav) return [];

    const ol = tocNav['ol'] as Record<string, unknown> | undefined;
    return parseNavList(ol, baseDir);
  }

  /** EPUB 2: parse the NCX `<navMap>` of nested `<navPoint>`s. */
  static fromNcx(xml: string | Uint8Array, baseDir: string): TocNode[] {
    const text = typeof xml === 'string' ? xml : decoder.decode(xml);
    const parsed = createXmlParser().parse(text) as Record<string, unknown>;
    const ncx = (parsed['ncx'] as Record<string, unknown>) ?? {};
    const navMap = ncx['navMap'] as Record<string, unknown> | undefined;
    return parseNavPoints(navMap?.['navPoint'], baseDir);
  }

  /** Fallback: one flat TOC entry per linear spine item. */
  static synthesize(
    spine: SpineItem[],
    labelFor: (item: SpineItem, index: number) => string,
  ): TocNode[] {
    return spine
      .filter((s) => s.linear)
      .map((s, i) => ({
        label: labelFor(s, i),
        href: s.item.href,
        children: [],
      }));
  }
}

function parseNavList(
  ol: Record<string, unknown> | undefined,
  baseDir: string,
): TocNode[] {
  if (!ol) return [];
  const items = toArray(ol['li']) as Array<Record<string, unknown>>;
  const nodes: TocNode[] = [];
  for (const li of items) {
    const anchor = (li['a'] ?? li['span']) as
      | Record<string, unknown>
      | string
      | undefined;
    const label = getText(anchor).trim();
    const hrefRaw =
      anchor && typeof anchor === 'object'
        ? (anchor['@_href'] as string | undefined)
        : undefined;

    const node: TocNode = { label, children: [] };
    if (hrefRaw) {
      try {
        node.href = PathResolver.resolveWithFragment(baseDir, hrefRaw);
      } catch {
        /* drop unresolvable href, keep the label */
      }
    }
    const childOl = li['ol'] as Record<string, unknown> | undefined;
    node.children = parseNavList(childOl, baseDir);
    // Skip empty structural <li> with neither label nor children.
    if (node.label || node.href || node.children.length) nodes.push(node);
  }
  return nodes;
}

function parseNavPoints(raw: unknown, baseDir: string): TocNode[] {
  const points = toArray(raw) as Array<Record<string, unknown>>;
  const nodes: TocNode[] = [];
  for (const np of points) {
    const navLabel = np['navLabel'] as Record<string, unknown> | undefined;
    const label = getText(navLabel?.['text']).trim();
    const content = np['content'] as Record<string, string> | undefined;
    const hrefRaw = content?.['@_src'];

    const node: TocNode = { label, children: [] };
    if (hrefRaw) {
      try {
        node.href = PathResolver.resolveWithFragment(baseDir, hrefRaw);
      } catch {
        /* drop unresolvable href */
      }
    }
    node.children = parseNavPoints(np['navPoint'], baseDir);
    nodes.push(node);
  }
  return nodes;
}
