import type { Command } from "../context";

export const challenge: Command = {
  name: "challenge",
  describe: "Get a random coding challenge from the catalog.",
  async run(ctx) {
    try {
      const c = await ctx.deps.catalog.random();
      if (!c) {
        await ctx.reply(
          "The challenge catalog is empty. Add one with `add <url>`.",
        );
        return;
      }
      await ctx.reply(`**${c.title}**\n${c.url}`);
    } catch {
      await ctx.reply("Sorry, I couldn't read the challenge catalog right now.");
    }
  },
};
