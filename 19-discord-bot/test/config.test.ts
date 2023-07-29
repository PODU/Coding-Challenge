import { describe, it, expect } from "vitest";
import { loadConfig } from "../src/config";

describe("loadConfig", () => {
  it("throws a clear error when the token is missing", () => {
    expect(() => loadConfig({} as NodeJS.ProcessEnv)).toThrow(
      /DISCORD_TOKEN is required/,
    );
  });

  it("applies defaults", () => {
    const cfg = loadConfig({ DISCORD_TOKEN: "abc" } as NodeJS.ProcessEnv);
    expect(cfg.prefix).toBe("!");
    expect(cfg.quotesApiUrl).toBe("https://dummyjson.com/quotes/random");
    expect(cfg.catalogPath).toBe("./data/challenges.json");
    expect(cfg.logLevel).toBe("info");
    expect(cfg.guildId).toBeUndefined();
  });

  it("requires app id only when asked", () => {
    expect(() =>
      loadConfig({ DISCORD_TOKEN: "abc" } as NodeJS.ProcessEnv, true),
    ).toThrow(/DISCORD_APP_ID is required/);
  });

  it("reads overrides", () => {
    const cfg = loadConfig({
      DISCORD_TOKEN: "abc",
      DISCORD_APP_ID: "123",
      DISCORD_GUILD_ID: "456",
      COMMAND_PREFIX: "?",
      LOG_LEVEL: "debug",
    } as NodeJS.ProcessEnv);
    expect(cfg.prefix).toBe("?");
    expect(cfg.guildId).toBe("456");
    expect(cfg.logLevel).toBe("debug");
  });
});
