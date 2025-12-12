/** Fetches imported scripts. Tests pass in their own fetch function. */

export type FetchLike = (
  input: string,
  init?: RequestInit,
) => Promise<{ ok: boolean; status: number; text(): Promise<string> }>;

export class NetworkError extends Error {
  constructor(
    message: string,
    readonly url: string,
    readonly status?: number,
  ) {
    super(message);
    this.name = "NetworkError";
  }
}

export class NetworkAdapter {
  private readonly doFetch: FetchLike;

  constructor(fetchImpl?: FetchLike) {
    if (fetchImpl) {
      this.doFetch = fetchImpl;
    } else if (typeof fetch !== "undefined") {
      this.doFetch = (input, init) => fetch(input, init);
    } else {
      this.doFetch = () => {
        throw new NetworkError("No fetch implementation available", "");
      };
    }
  }

  /** Return the body of url, or throw NetworkError. */
  async fetchText(url: string): Promise<string> {
    let res: Awaited<ReturnType<FetchLike>>;
    try {
      res = await this.doFetch(url, { redirect: "follow" });
    } catch (err) {
      throw new NetworkError(
        `Failed to fetch: ${(err as Error).message}`,
        url,
      );
    }
    if (!res.ok) {
      throw new NetworkError(
        `Request failed with status ${res.status}`,
        url,
        res.status,
      );
    }
    return res.text();
  }
}
