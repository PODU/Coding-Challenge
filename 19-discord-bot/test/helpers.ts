import { vi } from "vitest";
import type { CommandContext, CommandUser, Deps } from "../src/context";
import type { QuotesService } from "../src/services/quotes";
import type { CatalogService } from "../src/services/catalog";
import type { HttpService } from "../src/services/http";

export interface FakeContext extends CommandContext {
  replies: string[];
}

export function makeDeps(overrides: Partial<Deps> = {}): Deps {
  const quotes: QuotesService = {
    getRandomQuote: vi.fn(async () => ({ quote: "stub", author: "Stub" })),
  };
  const catalog: CatalogService = {
    list: vi.fn(async () => []),
    random: vi.fn(async () => undefined),
    add: vi.fn(async (input) => ({
      status: "added" as const,
      challenge: { ...input, addedAt: "1970-01-01T00:00:00.000Z" },
    })),
  };
  const http: HttpService = {
    fetchText: vi.fn(async () => "<title>Stub</title>"),
    extractTitle: vi.fn((html: string) => {
      const m = /<title>([^<]*)<\/title>/i.exec(html);
      return m ? m[1]!.trim() || null : null;
    }),
    isCodingChallengesUrl: vi.fn(() => true),
  };
  return { quotes, catalog, http, ...overrides };
}

export function makeContext(
  args: string[] = [],
  opts: { user?: Partial<CommandUser>; deps?: Partial<Deps> } = {},
): FakeContext {
  const replies: string[] = [];
  const user: CommandUser = {
    id: opts.user?.id ?? "1",
    username: opts.user?.username ?? "tester",
  };
  return {
    args,
    rawArgs: args.join(" "),
    user,
    deps: makeDeps(opts.deps),
    replies,
    reply: async (text: string) => {
      replies.push(text);
    },
  };
}
