import { describe, it, expect, vi } from "vitest";
import { add } from "../../src/commands/add";
import { makeContext } from "../helpers";
import type { CatalogService } from "../../src/services/catalog";
import type { HttpService } from "../../src/services/http";

const realHttp: HttpService = {
  // delegate to the real validators/parsers but never hit the network
  isCodingChallengesUrl: (url) => {
    try {
      const u = new URL(url);
      return (
        (u.protocol === "https:" || u.protocol === "http:") &&
        ["codingchallenges.fyi", "www.codingchallenges.fyi"].includes(
          u.hostname.toLowerCase(),
        )
      );
    } catch {
      return false;
    }
  },
  extractTitle: (html) => {
    const m = /<title>([^<]*)<\/title>/i.exec(html);
    return m ? m[1]!.replace(/\s+/g, " ").trim() || null : null;
  },
  fetchText: vi.fn(async () => "<title> My Challenge </title>"),
};

describe("add command", () => {
  it("requires a url argument", async () => {
    const ctx = makeContext([]);
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/usage/i);
  });

  it("rejects a non-codingchallenges host", async () => {
    const ctx = makeContext(["https://example.com/foo"], {
      deps: { http: realHttp },
    });
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/not a codingchallenges/i);
  });

  it("rejects a spoofed subdomain", async () => {
    const ctx = makeContext(
      ["https://codingchallenges.fyi.evil.com/x"],
      { deps: { http: realHttp } },
    );
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/not a codingchallenges/i);
  });

  it("fetches the page, extracts the title, and persists", async () => {
    const addFn = vi.fn(async (input) => ({
      status: "added" as const,
      challenge: { ...input, addedAt: "1970-01-01T00:00:00.000Z" },
    }));
    const catalog = {
      list: vi.fn(async () => []),
      random: vi.fn(async () => undefined),
      add: addFn,
    } as unknown as CatalogService;

    const ctx = makeContext(
      ["https://codingchallenges.fyi/challenges/challenge-wc"],
      { user: { username: "ada" }, deps: { http: realHttp, catalog } },
    );
    await add.run(ctx);

    expect(addFn).toHaveBeenCalledWith({
      title: "My Challenge",
      url: "https://codingchallenges.fyi/challenges/challenge-wc",
      addedBy: "ada",
    });
    expect(ctx.replies[0]).toContain("Added:");
    expect(ctx.replies[0]).toContain("My Challenge");
  });

  it("reports duplicates instead of re-adding", async () => {
    const catalog = {
      list: vi.fn(async () => []),
      random: vi.fn(async () => undefined),
      add: vi.fn(async () => ({
        status: "duplicate" as const,
        challenge: {
          title: "My Challenge",
          url: "https://codingchallenges.fyi/x",
          addedBy: "old",
          addedAt: "1970-01-01T00:00:00.000Z",
        },
      })),
    } as unknown as CatalogService;

    const ctx = makeContext(["https://codingchallenges.fyi/x"], {
      deps: { http: realHttp, catalog },
    });
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/already added/i);
  });

  it("rejects when the page has no title", async () => {
    const http: HttpService = {
      ...realHttp,
      fetchText: vi.fn(async () => "<html><body>no title here</body></html>"),
    };
    const ctx = makeContext(["https://codingchallenges.fyi/x"], {
      deps: { http },
    });
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/couldn't find a title/i);
  });

  it("handles a fetch failure gracefully", async () => {
    const http: HttpService = {
      ...realHttp,
      fetchText: vi.fn(async () => {
        throw new Error("boom");
      }),
    };
    const ctx = makeContext(["https://codingchallenges.fyi/x"], {
      deps: { http },
    });
    await add.run(ctx);
    expect(ctx.replies[0]).toMatch(/couldn't fetch/i);
  });
});
