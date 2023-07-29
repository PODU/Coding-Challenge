/** Configuration from the environment. Nothing else reads process.env. */
import * as dotenv from "dotenv";
import type { LogLevel } from "./logger";

dotenv.config();

export interface Config {
  token: string;
  appId: string;
  guildId: string | undefined;
  prefix: string;
  quotesApiUrl: string;
  catalogPath: string;
  logLevel: LogLevel;
}

const VALID_LEVELS: ReadonlySet<string> = new Set([
  "error",
  "warn",
  "info",
  "debug",
]);

function readLogLevel(raw: string | undefined): LogLevel {
  if (raw && VALID_LEVELS.has(raw)) return raw as LogLevel;
  return "info";
}

/**
 * Reads and validates config. Throws if DISCORD_TOKEN is missing, or
 * DISCORD_APP_ID when requireAppId is set (only slash registration needs it).
 */
export function loadConfig(
  env: NodeJS.ProcessEnv = process.env,
  requireAppId = false,
): Config {
  const token = (env.DISCORD_TOKEN ?? "").trim();
  if (!token) {
    throw new Error(
      "DISCORD_TOKEN is required but not set. Copy .env.example to .env and add " +
        "your bot token, or export DISCORD_TOKEN in the environment.",
    );
  }

  const appId = (env.DISCORD_APP_ID ?? "").trim();
  if (requireAppId && !appId) {
    throw new Error(
      "DISCORD_APP_ID is required to register slash commands. Set it in .env or " +
        "the environment (Discord Developer Portal -> General Information -> " +
        "Application ID).",
    );
  }

  return {
    token,
    appId,
    guildId: (env.DISCORD_GUILD_ID ?? "").trim() || undefined,
    prefix: (env.COMMAND_PREFIX ?? "!").trim() || "!",
    quotesApiUrl:
      (env.QUOTES_API_URL ?? "").trim() || "https://dummyjson.com/quotes/random",
    catalogPath: (env.CATALOG_PATH ?? "").trim() || "./data/challenges.json",
    logLevel: readLogLevel(env.LOG_LEVEL?.trim()),
  };
}
