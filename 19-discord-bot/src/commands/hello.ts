import type { Command } from "../context";

export const hello: Command = {
  name: "hello",
  describe: "Greet the bot and get a personalized hello back.",
  async run(ctx) {
    await ctx.reply(`Hello, ${ctx.user.username}!`);
  },
};
