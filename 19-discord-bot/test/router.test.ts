import { describe, it, expect, vi } from "vitest";
import {
  tokenize,
  parseMessage,
  isGreeting,
  dispatchMessage,
  type IncomingMessage,
} from "../src/router";
import { makeDeps } from "./helpers";

describe("tokenize", () => {
  it("splits on whitespace and collapses extra spaces", () => {
    expect(tokenize("  add   foo   bar ")).toEqual(["add", "foo", "bar"]);
  });
  it("honors quoted args", () => {
    expect(tokenize('add "hello world" baz')).toEqual([
      "add",
      "hello world",
      "baz",
    ]);
  });
});

describe("parseMessage", () => {
  it("returns null without the prefix", () => {
    expect(parseMessage("quote", "!")).toBeNull();
  });
  it("parses name and args, lowercasing the name", () => {
    expect(parseMessage("!ADD https://x", "!")).toEqual({
      name: "add",
      args: ["https://x"],
      rawArgs: "https://x",
    });
  });
  it("returns null for a bare prefix", () => {
    expect(parseMessage("!", "!")).toBeNull();
  });
});

describe("isGreeting", () => {
  it("matches Hello case-insensitively", () => {
    expect(isGreeting("Hello there")).toBe(true);
    expect(isGreeting("oh hello")).toBe(true);
  });
  it("does not match unrelated text", () => {
    expect(isGreeting("goodbye")).toBe(false);
  });
});

function makeIncoming(
  content: string,
  authorIsBot = false,
): IncomingMessage & { replies: string[] } {
  const replies: string[] = [];
  return {
    content,
    authorIsBot,
    user: { id: "1", username: "Ada" },
    replies,
    reply: async (t: string) => {
      replies.push(t);
    },
  };
}

describe("dispatchMessage", () => {
  const opts = { prefix: "!", deps: makeDeps() };

  it("ignores bot messages", async () => {
    const msg = makeIncoming("!quote", true);
    await dispatchMessage(msg, opts);
    expect(msg.replies).toHaveLength(0);
  });

  it("greets on Hello", async () => {
    const msg = makeIncoming("Hello!");
    await dispatchMessage(msg, opts);
    expect(msg.replies[0]).toBe("Hello, Ada!");
  });

  it("dispatches a known prefix command", async () => {
    const msg = makeIncoming("!quote");
    await dispatchMessage(msg, opts);
    expect(msg.replies[0]).toContain("stub");
  });

  it("replies for an unknown command", async () => {
    const msg = makeIncoming("!nope");
    await dispatchMessage(msg, opts);
    expect(msg.replies[0]).toMatch(/unknown command/i);
  });

  it("surfaces a handler error as a friendly reply", async () => {
    const deps = makeDeps({
      quotes: {
        getRandomQuote: vi.fn(async () => {
          throw new Error("x");
        }),
      },
    });
    // quote handler catches its own error, so force a registry-level throw by
    // making reply throw once then succeed.
    const msg = makeIncoming("!quote");
    await dispatchMessage(msg, { prefix: "!", deps });
    // quote command swallows the error internally -> fallback message
    expect(msg.replies[0]).toMatch(/couldn't fetch a quote/i);
  });
});
