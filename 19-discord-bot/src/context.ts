/**
 * Commands only see a CommandContext, never discord.js objects, so they can be
 * tested without a gateway connection.
 */
import type { QuotesService } from "./services/quotes";
import type { CatalogService } from "./services/catalog";
import type { HttpService } from "./services/http";

export interface CommandUser {
  id: string;
  username: string;
}

export interface Deps {
  quotes: QuotesService;
  catalog: CatalogService;
  http: HttpService;
}

export interface CommandContext {
  args: string[];
  /** Everything after the command name, trimmed. */
  rawArgs: string;
  user: CommandUser;
  reply(text: string): Promise<void>;
  deps: Deps;
}

export interface Command {
  /** Lowercase, without the prefix. */
  name: string;
  /** Also used as the slash-command description. */
  describe: string;
  /** Should report failures through reply() rather than throw. */
  run(ctx: CommandContext): Promise<void>;
}

/** Discord's per-message character limit. */
export const DISCORD_MAX_MESSAGE = 2000;
