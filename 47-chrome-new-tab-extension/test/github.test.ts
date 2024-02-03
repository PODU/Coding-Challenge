import { describe, expect, it, vi } from 'vitest';
import {
  fetchOpenPulls,
  GitHubHttpError,
  mapPulls,
  type FetchLike,
  type FetchResponse,
} from '../src/github.js';

const FIXTURE = [
  {
    number: 12,
    title: 'Add Go solution for challenge 7',
    html_url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/12',
    user: { login: 'alice' },
  },
  {
    number: 13,
    title: 'Fix typo in README',
    html_url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/13',
    user: { login: 'bob' },
  },
];

function ok(body: unknown): FetchResponse {
  return { ok: true, status: 200, json: () => Promise.resolve(body) };
}
function fail(status: number): FetchResponse {
  return { ok: false, status, json: () => Promise.resolve({}) };
}

describe('mapPulls', () => {
  it('maps a canned GitHub JSON fixture to PullRequest[]', () => {
    expect(mapPulls(FIXTURE)).toEqual([
      {
        number: 12,
        title: 'Add Go solution for challenge 7',
        url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/12',
        author: 'alice',
      },
      {
        number: 13,
        title: 'Fix typo in README',
        url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/13',
        author: 'bob',
      },
    ]);
  });

  it('drops entries missing required fields (title / html_url / number)', () => {
    const raw = [
      { number: 1, html_url: 'https://x/1', user: { login: 'a' } }, // no title
      { number: 2, title: 'has no url', user: { login: 'b' } }, // no html_url
      { title: 'no number', html_url: 'https://x/3', user: { login: 'c' } }, // no number
      { number: 4, title: 'good', html_url: 'https://x/4', user: { login: 'd' } },
    ];
    expect(mapPulls(raw)).toEqual([
      { number: 4, title: 'good', url: 'https://x/4', author: 'd' },
    ]);
  });

  it('falls back to "unknown" author when user/login is absent', () => {
    const raw = [{ number: 9, title: 't', html_url: 'https://x/9' }];
    expect(mapPulls(raw)[0]?.author).toBe('unknown');
  });

  it('returns [] for non-array input', () => {
    expect(mapPulls(null)).toEqual([]);
    expect(mapPulls({})).toEqual([]);
  });
});

describe('fetchOpenPulls', () => {
  it('200 -> parsed PullRequest[] and calls the expected endpoint', async () => {
    const fakeFetch = vi.fn<Parameters<FetchLike>, ReturnType<FetchLike>>(() =>
      Promise.resolve(ok(FIXTURE)),
    );
    const prs = await fetchOpenPulls('CodingChallengesFYI', 'SharedSolutions', fakeFetch);

    expect(prs).toHaveLength(2);
    expect(prs[0]?.author).toBe('alice');
    const url = fakeFetch.mock.calls[0]?.[0] as string;
    expect(url).toContain(
      'https://api.github.com/repos/CodingChallengesFYI/SharedSolutions/pulls',
    );
    expect(url).toContain('state=open');
  });

  it('403 -> throws a typed rate-limit error (no real HTTP)', async () => {
    const fakeFetch: FetchLike = () => Promise.resolve(fail(403));
    await expect(fetchOpenPulls('o', 'r', fakeFetch)).rejects.toBeInstanceOf(
      GitHubHttpError,
    );
    await expect(fetchOpenPulls('o', 'r', fakeFetch)).rejects.toThrow(/rate limit/i);
  });

  it('non-200 -> throws GitHubHttpError carrying the status', async () => {
    const fakeFetch: FetchLike = () => Promise.resolve(fail(500));
    await expect(fetchOpenPulls('o', 'r', fakeFetch)).rejects.toMatchObject({
      status: 500,
    });
  });

  it('network rejection propagates (e.g. offline)', async () => {
    const fakeFetch: FetchLike = () => Promise.reject(new Error('Failed to fetch'));
    await expect(fetchOpenPulls('o', 'r', fakeFetch)).rejects.toThrow('Failed to fetch');
  });
});
