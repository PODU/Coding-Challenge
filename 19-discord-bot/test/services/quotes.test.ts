import { describe, it, expect, vi } from "vitest";
import { createQuotesService } from "../../src/services/quotes";

const URL = "https://dummyjson.com/quotes/random";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

describe("quotes service", () => {
  it("returns quote and author on 200", async () => {
    const fetchImpl = vi.fn(async () =>
      jsonResponse({ quote: "Be excellent.", author: "Bill" }),
    );
    const svc = createQuotesService(URL, fetchImpl as unknown as typeof fetch);
    await expect(svc.getRandomQuote()).resolves.toEqual({
      quote: "Be excellent.",
      author: "Bill",
    });
  });

  it("defaults a missing author to Unknown", async () => {
    const fetchImpl = vi.fn(async () => jsonResponse({ quote: "Hi" }));
    const svc = createQuotesService(URL, fetchImpl as unknown as typeof fetch);
    const q = await svc.getRandomQuote();
    expect(q.author).toBe("Unknown");
  });

  it("throws on a 500", async () => {
    const fetchImpl = vi.fn(async () => jsonResponse({}, 500));
    const svc = createQuotesService(URL, fetchImpl as unknown as typeof fetch);
    await expect(svc.getRandomQuote()).rejects.toThrow(/status 500/);
  });

  it("throws on a malformed body", async () => {
    const fetchImpl = vi.fn(async () => jsonResponse({ author: "x" }));
    const svc = createQuotesService(URL, fetchImpl as unknown as typeof fetch);
    await expect(svc.getRandomQuote()).rejects.toThrow(/malformed/);
  });
});
