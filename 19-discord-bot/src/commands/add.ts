import type { Command } from "../context";

export const add: Command = {
  name: "add",
  describe: "Add a challenge from a codingchallenges.fyi URL.",
  async run(ctx) {
    const url = ctx.args[0];
    if (!url) {
      await ctx.reply("Usage: `add <url>` (must be a codingchallenges.fyi URL).");
      return;
    }

    const { http, catalog } = ctx.deps;

    if (!http.isCodingChallengesUrl(url)) {
      await ctx.reply(
        `Rejected: \`${url}\` is not a codingchallenges.fyi URL.`,
      );
      return;
    }

    let html: string;
    try {
      html = await http.fetchText(url);
    } catch {
      await ctx.reply(
        `Couldn't fetch that page. Check the URL is reachable and try again.`,
      );
      return;
    }

    const title = http.extractTitle(html);
    if (!title) {
      await ctx.reply(
        "Couldn't find a title on that page, so I didn't add it.",
      );
      return;
    }

    try {
      const result = await catalog.add({
        title,
        url,
        addedBy: ctx.user.username,
      });
      if (result.status === "duplicate") {
        await ctx.reply(`Already added: **${result.challenge.title}**`);
      } else {
        await ctx.reply(`Added: **${result.challenge.title}**\n${result.challenge.url}`);
      }
    } catch {
      await ctx.reply("Couldn't save that challenge right now. Try again later.");
    }
  },
};
