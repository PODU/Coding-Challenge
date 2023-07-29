import { describe, it, expect, vi } from "vitest";
import { list, formatList } from "../../src/commands/list";
import { makeContext } from "../helpers";
import { DISCORD_MAX_MESSAGE } from "../../src/context";
import type { CatalogService } from "../../src/services/catalog";

describe("formatList", () => {
  it("renders all entries", () => {
    const out = formatList([
      { title: "A", url: "https://codingchallenges.fyi/a" },
      { title: "B", url: "https://codingchallenges.fyi/b" },
    ]);
    expect(out).toContain("Challenges (2)");
    expect(out).toContain("1. A");
    expect(out).toContain("2. B");
  });

  it("reports an empty catalog", () => {
    expect(formatList([])).toMatch(/empty/i);
  });

  it("truncates past the 2000-char limit", () => {
    const many = Array.from({ length: 500 }, (_, i) => ({
      title: `Challenge number ${i} with a reasonably long title`,
      url: `https://codingchallenges.fyi/challenges/challenge-${i}`,
    }));
    const out = formatList(many);
    expect(out.length).toBeLessThanOrEqual(DISCORD_MAX_MESSAGE);
    expect(out).toMatch(/truncated/i);
  });
});

describe("list command", () => {
  it("replies with the rendered catalog", async () => {
    const catalog = {
      list: vi.fn(async () => [
        {
          title: "wc",
          url: "https://codingchallenges.fyi/wc",
          addedBy: "seed",
          addedAt: "1970-01-01T00:00:00.000Z",
        },
      ]),
      random: vi.fn(),
      add: vi.fn(),
    } as unknown as CatalogService;
    const ctx = makeContext([], { deps: { catalog } });
    await list.run(ctx);
    expect(ctx.replies[0]).toContain("wc");
  });
});
