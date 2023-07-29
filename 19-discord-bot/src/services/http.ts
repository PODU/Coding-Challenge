/**
 * HTTP helpers: fetch with a timeout, the codingchallenges.fyi host check, and
 * page title extraction. Uses the global fetch (Node 18+), which tests stub.
 */
import * as cheerio from "cheerio";

export interface FetchOptions {
  timeoutMs?: number;
  headers?: Record<string, string>;
}

export interface HttpService {
  /** Throws on a non-2xx response. */
  fetchText(url: string, opts?: FetchOptions): Promise<string>;
  /** The page <title> with whitespace collapsed, or null if there isn't one. */
  extractTitle(html: string): string | null;
  /** True only for codingchallenges.fyi or www.codingchallenges.fyi. */
  isCodingChallengesUrl(url: string): boolean;
}

const DEFAULT_TIMEOUT_MS = 8000;
const ALLOWED_HOSTS: ReadonlySet<string> = new Set([
  "codingchallenges.fyi",
  "www.codingchallenges.fyi",
]);

export function isCodingChallengesUrl(url: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return false;
  }
  // Exact match, so "codingchallenges.fyi.evil.com" and
  // "evilcodingchallenges.fyi" are rejected.
  return ALLOWED_HOSTS.has(parsed.hostname.toLowerCase());
}

export function extractTitle(html: string): string | null {
  const $ = cheerio.load(html);
  const title = $("title").first().text().replace(/\s+/g, " ").trim();
  return title || null;
}

export async function fetchText(
  url: string,
  opts: FetchOptions = {},
): Promise<string> {
  const controller = new AbortController();
  const timeout = setTimeout(
    () => controller.abort(),
    opts.timeoutMs ?? DEFAULT_TIMEOUT_MS,
  );
  try {
    const res = await fetch(url, {
      signal: controller.signal,
      headers: {
        "user-agent": "disdaccbot/1.0 (+https://codingchallenges.fyi)",
        ...(opts.headers ?? {}),
      },
    });
    if (!res.ok) {
      throw new Error(`Request to ${url} failed with status ${res.status}`);
    }
    return await res.text();
  } finally {
    clearTimeout(timeout);
  }
}

export function createHttpService(): HttpService {
  return { fetchText, extractTitle, isCodingChallengesUrl };
}
