import { describe, expect, it } from 'vitest';
import { parseRss } from '../src/rss.js';

const FIXTURE = `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Coding Challenges</title>
    <item>
      <title><![CDATA[Coding Challenge #47 — Build your own Chrome Extension]]></title>
      <link>https://codingchallenges.substack.com/p/coding-challenge-47</link>
      <pubDate>Fri, 26 Jun 2026 09:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Challenge #46 &amp; friends</title>
      <link>https://codingchallenges.substack.com/p/coding-challenge-46</link>
      <pubDate>Fri, 19 Jun 2026 09:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`;

describe('parseRss', () => {
  it('extracts title/link/pubDate from each item', () => {
    const items = parseRss(FIXTURE);
    expect(items).toHaveLength(2);
    expect(items[0]).toEqual({
      title: 'Coding Challenge #47 — Build your own Chrome Extension',
      link: 'https://codingchallenges.substack.com/p/coding-challenge-47',
      pubDate: 'Fri, 26 Jun 2026 09:00:00 GMT',
    });
  });

  it('decodes XML entities in titles', () => {
    const items = parseRss(FIXTURE);
    expect(items[1]?.title).toBe('Challenge #46 & friends');
  });

  it('returns [] for an empty or item-less document', () => {
    expect(parseRss('')).toEqual([]);
    expect(parseRss('<rss><channel></channel></rss>')).toEqual([]);
  });
});
