import { describe, it, expect, vi, afterEach } from "vitest";
import {
  isCodingChallengesUrl,
  extractTitle,
  fetchText,
} from "../../src/services/http";

describe("isCodingChallengesUrl", () => {
  const good = [
    "https://codingchallenges.fyi/",
    "https://codingchallenges.fyi/challenges/challenge-wc",
    "http://codingchallenges.fyi/x",
    "https://www.codingchallenges.fyi/x",
    "https://CodingChallenges.fyi/x",
  ];
  const bad = [
    "https://codingchallenges.fyi.evil.com/x",
    "https://evilcodingchallenges.fyi/x",
    "https://example.com/x",
    "ftp://codingchallenges.fyi/x",
    "not a url",
    "",
  ];

  it.each(good)("accepts %s", (u) => {
    expect(isCodingChallengesUrl(u)).toBe(true);
  });
  it.each(bad)("rejects %s", (u) => {
    expect(isCodingChallengesUrl(u)).toBe(false);
  });
});

describe("extractTitle", () => {
  it("extracts a normal title", () => {
    expect(extractTitle("<html><title>Hello</title></html>")).toBe("Hello");
  });
  it("collapses whitespace", () => {
    expect(
      extractTitle("<title>  Write  Your\n Own  wc </title>"),
    ).toBe("Write Your Own wc");
  });
  it("returns null when missing", () => {
    expect(extractTitle("<html><body>no title</body></html>")).toBeNull();
  });
  it("returns null for an empty title", () => {
    expect(extractTitle("<title>   </title>")).toBeNull();
  });
});

describe("fetchText", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns the body on 200", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("<title>OK</title>", { status: 200 })),
    );
    await expect(fetchText("https://codingchallenges.fyi/x")).resolves.toContain(
      "OK",
    );
  });

  it("throws on a non-2xx response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 500 })),
    );
    await expect(
      fetchText("https://codingchallenges.fyi/x"),
    ).rejects.toThrow(/status 500/);
  });
});
