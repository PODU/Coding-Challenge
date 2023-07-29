import { DISCORD_MAX_MESSAGE, type Command } from "../context";

const ELLIPSIS_NOTE = "\n… (list truncated)";

/** Format the catalog, cut short if it would exceed Discord's message limit. */
export function formatList(
  entries: { title: string; url: string }[],
): string {
  if (entries.length === 0) {
    return "The challenge catalog is empty. Add one with `add <url>`.";
  }

  const header = `**Challenges (${entries.length}):**`;
  const lines: string[] = [header];
  let length = header.length;
  let truncated = false;

  for (let i = 0; i < entries.length; i++) {
    const e = entries[i]!;
    const line = `${i + 1}. ${e.title} — <${e.url}>`;
    // +1 for the joining newline, and leave room for the truncation note.
    if (length + line.length + 1 > DISCORD_MAX_MESSAGE - ELLIPSIS_NOTE.length) {
      truncated = true;
      break;
    }
    lines.push(line);
    length += line.length + 1;
  }

  let out = lines.join("\n");
  if (truncated) out += ELLIPSIS_NOTE;
  return out;
}

export const list: Command = {
  name: "list",
  describe: "List all available coding challenges.",
  async run(ctx) {
    try {
      const entries = await ctx.deps.catalog.list();
      await ctx.reply(formatList(entries));
    } catch {
      await ctx.reply("Sorry, I couldn't read the challenge catalog right now.");
    }
  },
};
