import type { Command } from "../context";

export const quote: Command = {
  name: "quote",
  describe: "Fetch a random quote.",
  async run(ctx) {
    try {
      const q = await ctx.deps.quotes.getRandomQuote();
      await ctx.reply(`> ${q.quote}\n— ${q.author}`);
    } catch {
      await ctx.reply("Sorry, I couldn't fetch a quote right now. Try again later.");
    }
  },
};
