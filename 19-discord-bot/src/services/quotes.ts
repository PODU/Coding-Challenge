/** Random quotes from the dummyJSON API. */
export interface Quote {
  quote: string;
  author: string;
}

export interface QuotesService {
  getRandomQuote(): Promise<Quote>;
}

interface DummyJsonQuote {
  id?: number;
  quote?: unknown;
  author?: unknown;
}

const DEFAULT_TIMEOUT_MS = 8000;

export function createQuotesService(
  apiUrl: string,
  fetchImpl: typeof fetch = fetch,
): QuotesService {
  return {
    async getRandomQuote(): Promise<Quote> {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);
      try {
        const res = await fetchImpl(apiUrl, { signal: controller.signal });
        if (!res.ok) {
          throw new Error(`Quotes API responded with status ${res.status}`);
        }
        const body = (await res.json()) as DummyJsonQuote;
        const quote = typeof body.quote === "string" ? body.quote.trim() : "";
        const author =
          typeof body.author === "string" && body.author.trim().length > 0
            ? body.author.trim()
            : "Unknown";
        if (!quote) {
          throw new Error("Quotes API returned a malformed body (no quote text)");
        }
        return { quote, author };
      } finally {
        clearTimeout(timeout);
      }
    },
  };
}
