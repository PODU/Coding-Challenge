/** Fetching open pull requests from the GitHub API. */

export interface PullRequest {
  title: string;
  author: string;
  url: string;
  number: number;
}

/** The part of `fetch` we use, so tests can pass a fake. */
export type FetchLike = (
  input: string,
  init?: RequestInit,
) => Promise<FetchResponse>;

export interface FetchResponse {
  ok: boolean;
  status: number;
  json(): Promise<unknown>;
}

export class GitHubHttpError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'GitHubHttpError';
    this.status = status;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

/**
 * Maps GitHub `/pulls` JSON to `PullRequest[]`, skipping entries that lack a
 * title, URL or number.
 */
export function mapPulls(raw: unknown): PullRequest[] {
  if (!Array.isArray(raw)) return [];
  const out: PullRequest[] = [];

  for (const entry of raw) {
    if (!isRecord(entry)) continue;

    const title = entry['title'];
    const url = entry['html_url'];
    const num = entry['number'];
    const user = entry['user'];
    const author = isRecord(user) ? user['login'] : undefined;

    if (typeof title !== 'string' || title.length === 0) continue;
    if (typeof url !== 'string' || url.length === 0) continue;
    if (typeof num !== 'number') continue;

    out.push({
      title,
      url,
      number: num,
      author: typeof author === 'string' && author.length > 0 ? author : 'unknown',
    });
  }

  return out;
}

/**
 * Fetches open pull requests for `owner/repo`. Throws GitHubHttpError on a
 * non-2xx response; network failures propagate as-is.
 */
export async function fetchOpenPulls(
  owner: string,
  repo: string,
  fetchImpl: FetchLike = fetch as unknown as FetchLike,
): Promise<PullRequest[]> {
  const endpoint =
    `https://api.github.com/repos/${encodeURIComponent(owner)}/` +
    `${encodeURIComponent(repo)}/pulls?state=open&per_page=20`;

  const res = await fetchImpl(endpoint, {
    headers: { Accept: 'application/vnd.github+json' },
  });

  if (!res.ok) {
    const reason =
      res.status === 403 || res.status === 429
        ? 'GitHub API rate limit reached'
        : `GitHub API returned HTTP ${res.status}`;
    throw new GitHubHttpError(res.status, reason);
  }

  return mapPulls(await res.json());
}
