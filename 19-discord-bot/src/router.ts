/**
 * Routes prefixed text commands and replies to "hello". Works on a plain
 * IncomingMessage; src/client.ts converts discord.js messages into one.
 */
import type { CommandContext, CommandUser, Deps } from "./context";
import { commandRegistry, hello } from "./commands";
import type { Logger } from "./logger";

export interface IncomingMessage {
  content: string;
  authorIsBot: boolean;
  user: CommandUser;
  reply(text: string): Promise<void>;
}

export interface ParsedCommand {
  name: string;
  args: string[];
  rawArgs: string;
}

export interface RouterOptions {
  prefix: string;
  deps: Deps;
  logger?: Logger;
}

/** Split on whitespace, keeping single- or double-quoted args together. */
export function tokenize(input: string): string[] {
  const tokens: string[] = [];
  const re = /"([^"]*)"|'([^']*)'|(\S+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(input)) !== null) {
    tokens.push(m[1] ?? m[2] ?? m[3] ?? "");
  }
  return tokens;
}

/** Parse a prefixed message into a command, or null if it isn't one. */
export function parseMessage(
  content: string,
  prefix: string,
): ParsedCommand | null {
  if (!content.startsWith(prefix)) return null;
  const body = content.slice(prefix.length).trim();
  const tokens = tokenize(body);
  if (tokens.length === 0) return null;
  return {
    name: tokens[0]!.toLowerCase(),
    args: tokens.slice(1),
    rawArgs: body.slice(tokens[0]!.length).trim(),
  };
}

/** True if the message contains the word "hello", in any case. */
export function isGreeting(content: string): boolean {
  return /\bhello\b/i.test(content);
}

/** Handle one message. Errors are logged and answered, never thrown. */
export async function dispatchMessage(
  msg: IncomingMessage,
  opts: RouterOptions,
): Promise<void> {
  const { prefix, deps, logger } = opts;

  if (msg.authorIsBot) return;

  const buildCtx = (args: string[], rawArgs: string): CommandContext => ({
    args,
    rawArgs,
    user: msg.user,
    reply: msg.reply,
    deps,
  });

  // "!hello" is a command, not a greeting.
  if (msg.content.startsWith(prefix)) {
    const parsed = parseMessage(msg.content, prefix);
    if (!parsed) {
      await msg.reply(`Unknown command. Try \`${prefix}quote\`, \`${prefix}challenge\`, \`${prefix}add <url>\`, or \`${prefix}list\`.`);
      return;
    }
    const command = commandRegistry.get(parsed.name);
    if (!command) {
      await msg.reply(
        `Unknown command \`${parsed.name}\`. Try \`${prefix}quote\`, \`${prefix}challenge\`, \`${prefix}add <url>\`, or \`${prefix}list\`.`,
      );
      return;
    }
    try {
      await command.run(buildCtx(parsed.args, parsed.rawArgs));
    } catch (err) {
      logger?.error("Command handler threw", {
        command: command.name,
        error: (err as Error).message,
      });
      await msg.reply("Something went wrong handling that command.");
    }
    return;
  }

  if (isGreeting(msg.content)) {
    try {
      await hello.run(buildCtx([], ""));
    } catch (err) {
      logger?.error("Greeting handler threw", {
        error: (err as Error).message,
      });
    }
  }
}
