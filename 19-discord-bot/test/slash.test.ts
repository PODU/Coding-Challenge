import { describe, it, expect, vi } from "vitest";
import { dispatchInteraction, type SlashInteraction } from "../src/slash";
import { makeDeps } from "./helpers";

function makeInteraction(
  commandName: string,
  options: Record<string, string> = {},
): SlashInteraction & { replies: string[] } {
  const replies: string[] = [];
  return {
    commandName,
    user: { id: "1", username: "Ada" },
    getStringOption: (name: string) => options[name] ?? null,
    replies,
    reply: async (t: string) => {
      replies.push(t);
    },
  };
}

describe("dispatchInteraction", () => {
  const deps = makeDeps();

  it("dispatches /quote into the shared handler", async () => {
    const i = makeInteraction("quote");
    await dispatchInteraction(i, { deps });
    expect(i.replies[0]).toContain("stub");
  });

  it("passes the url option through to /add", async () => {
    const addFn = vi.fn(async (input) => ({
      status: "added" as const,
      challenge: { ...input, addedAt: "1970-01-01T00:00:00.000Z" },
    }));
    const d = makeDeps({
      catalog: {
        list: vi.fn(async () => []),
        random: vi.fn(async () => undefined),
        add: addFn,
      },
    });
    const i = makeInteraction("add", {
      url: "https://codingchallenges.fyi/x",
    });
    await dispatchInteraction(i, { deps: d });
    expect(addFn).toHaveBeenCalledOnce();
    expect(i.replies[0]).toContain("Added:");
  });

  it("replies for an unknown command", async () => {
    const i = makeInteraction("bogus");
    await dispatchInteraction(i, { deps });
    expect(i.replies[0]).toMatch(/unknown command/i);
  });
});
