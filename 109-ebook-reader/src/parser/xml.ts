import { XMLParser } from 'fast-xml-parser';

/**
 * Shared, deterministic XML parser configuration used by every parser stage.
 *
 * `removeNSPrefix` collapses namespace prefixes (`dc:title` -> `title`,
 * `opf:role` -> `role`, `epub:type` -> `type`) so downstream code does not have
 * to guess which prefix a given file used.
 */
export function createXmlParser(): XMLParser {
  return new XMLParser({
    ignoreAttributes: false,
    attributeNamePrefix: '@_',
    removeNSPrefix: true,
    parseAttributeValue: false,
    parseTagValue: false,
    trimValues: true,
    textNodeName: '#text',
    // Repeated elements always become arrays so order/shape is predictable.
    isArray: (name) =>
      [
        'item',
        'itemref',
        'rootfile',
        'creator',
        'identifier',
        'meta',
        'navPoint',
        'nav',
        'li',
      ].includes(name),
  });
}

/** Coerce a value into an array (treating null/undefined as empty). */
export function toArray<T>(x: T | T[] | undefined | null): T[] {
  if (x == null) return [];
  return Array.isArray(x) ? x : [x];
}

/** Extract text content from a parsed node (string, or object with `#text`). */
export function getText(node: unknown): string {
  if (node == null) return '';
  if (typeof node === 'string') return node;
  if (typeof node === 'number') return String(node);
  if (typeof node === 'object') {
    const t = (node as Record<string, unknown>)['#text'];
    if (t != null) return String(t);
  }
  return '';
}
