import { describe, it, expect, vi } from "vitest";
import { quote } from "../../src/commands/quote";
import { makeContext } from "../helpers";

describe("quote command", () => {
  it("formats the quote from the service", async () => {
    const ctx = makeContext([], {
      deps: {
        quotes: {
          getRandomQuote: vi.fn(async () => ({
            quote: "Stay hungry.",
            author: "Steve",
          })),
        },
      },
    });
    await quote.run(ctx);
    expect(ctx.replies[0]).toContain("Stay hungry.");
    expect(ctx.replies[0]).toContain("Steve");
  });

  it("falls back gracefully when the service throws", async () => {
    const ctx = makeContext([], {
      deps: {
        quotes: {
          getRandomQuote: vi.fn(async () => {
            throw new Error("network down");
          }),
        },
      },
    });
    await quote.run(ctx);
    expect(ctx.replies[0]).toMatch(/couldn't fetch a quote/i);
  });
});
