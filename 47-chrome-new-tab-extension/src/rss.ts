/**
 * Regex-based RSS parser. It avoids DOMParser so the same code runs in the
 * page, a service worker and Node tests. Fetching the feed is not handled here.
 */

export interface FeedItem {
  title: string;
  link: string;
  pubDate?: string;
}

const ITEM_RE = /<item\b[^>]*>([\s\S]*?)<\/item>/gi;

/** Text of the first `<tag>...</tag>` in block, with CDATA unwrapped. */
function extractTag(block: string, tag: string): string | undefined {
  const re = new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)<\\/${tag}>`, 'i');
  const m = re.exec(block);
  if (!m || m[1] === undefined) return undefined;
  return decodeXml(stripCdata(m[1]).trim());
}

function stripCdata(s: string): string {
  const cdata = /^\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*$/.exec(s);
  return cdata && cdata[1] !== undefined ? cdata[1] : s;
}

function decodeXml(s: string): string {
  return s
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&apos;/g, "'")
    .replace(/&amp;/g, '&');
}

/** Parses RSS items, skipping any that have neither a title nor a link. */
export function parseRss(xml: string): FeedItem[] {
  const items: FeedItem[] = [];
  if (!xml) return items;

  let match: RegExpExecArray | null;
  ITEM_RE.lastIndex = 0;
  while ((match = ITEM_RE.exec(xml)) !== null) {
    const block = match[1] ?? '';
    const title = extractTag(block, 'title');
    const link = extractTag(block, 'link');
    const pubDate = extractTag(block, 'pubDate');

    if (title === undefined && link === undefined) continue;

    const item: FeedItem = {
      title: title ?? '(untitled)',
      link: link ?? '',
    };
    if (pubDate !== undefined) item.pubDate = pubDate;
    items.push(item);
  }

  return items;
}
