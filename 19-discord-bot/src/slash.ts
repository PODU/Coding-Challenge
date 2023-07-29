/**
 * Runs slash commands through the same command registry as the prefix router,
 * so /quote and !quote share one implementation. src/client.ts converts
 * discord.js interactions into a SlashInteraction.
 */
import type { CommandContext, CommandUser, Deps } from "./context";
import { commandRegistry } from "./commands";
import type { Logger } from "./logger";

export interface SlashInteraction {
  commandName: string;
  user: CommandUser;
  /** null when the option wasn't given. */
  getStringOption(name: string): string | null;
  reply(text: string): Promise<void>;
}

export interface SlashOptions {
  deps: Deps;
  logger?: Logger;
}

export async function dispatchInteraction(
  interaction: SlashInteraction,
  opts: SlashOptions,
): Promise<void> {
  const { deps, logger } = opts;
  const command = commandRegistry.get(interaction.commandName.toLowerCase());
  if (!command) {
    await interaction.reply(`Unknown command \`${interaction.commandName}\`.`);
    return;
  }

  // Only /add takes an argument so far.
  const url = interaction.getStringOption("url");
  const ctx: CommandContext = {
    args: url ? [url] : [],
    rawArgs: url ?? "",
    user: interaction.user satisfies CommandUser,
    reply: interaction.reply.bind(interaction),
    deps,
  };

  try {
    await command.run(ctx);
  } catch (err) {
    logger?.error("Slash handler threw", {
      command: command.name,
      error: (err as Error).message,
    });
    await interaction.reply("Something went wrong handling that command.");
  }
}
