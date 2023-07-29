import { describe, it, expect, vi } from "vitest";
import { challenge } from "../../src/commands/challenge";
import { makeContext } from "../helpers";

describe("challenge command", () => {
  it("returns a random catalog entry", async () => {
    const ctx = makeContext([], {
      deps: {
        catalog: {
          list: vi.fn(async () => []),
          add: vi.fn(),
          random: vi.fn(async () => ({
            title: "Write Your Own wc",
            url: "https://codingchallenges.fyi/challenges/challenge-wc",
            addedBy: "seed",
            addedAt: "1970-01-01T00:00:00.000Z",
          })),
        } as never,
      },
    });
    await challenge.run(ctx);
    expect(ctx.replies[0]).toContain("Write Your Own wc");
    expect(ctx.replies[0]).toContain("challenge-wc");
  });

  it("handles an empty catalog", async () => {
    const ctx = makeContext([], {
      deps: {
        catalog: {
          list: vi.fn(async () => []),
          add: vi.fn(),
          random: vi.fn(async () => undefined),
        } as never,
      },
    });
    await challenge.run(ctx);
    expect(ctx.replies[0]).toMatch(/empty/i);
  });
});
