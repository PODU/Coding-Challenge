/**
 * Registers the slash commands. Run with `npm run register`, or
 * `npm run register:built` after a build.
 *
 * With DISCORD_GUILD_ID set the commands go to that guild and show up at once,
 * which is handy in development. Otherwise they are registered globally, which
 * can take up to an hour to appear.
 */
import {
  REST,
  Routes,
  SlashCommandBuilder,
  type RESTPostAPIApplicationCommandsJSONBody,
} from "discord.js";
import { loadConfig } from "./config";
import { createLogger } from "./logger";
import { slashCommands } from "./commands";

function buildCommandPayload(): RESTPostAPIApplicationCommandsJSONBody[] {
  return slashCommands.map((cmd) => {
    const builder = new SlashCommandBuilder()
      .setName(cmd.name)
      .setDescription(cmd.describe);
    if (cmd.name === "add") {
      builder.addStringOption((opt) =>
        opt
          .setName("url")
          .setDescription("A codingchallenges.fyi challenge URL")
          .setRequired(true),
      );
    }
    return builder.toJSON();
  });
}

async function main(): Promise<void> {
  const config = loadConfig(process.env, true);
  const logger = createLogger(config.logLevel);
  const rest = new REST({ version: "10" }).setToken(config.token);
  const body = buildCommandPayload();

  if (config.guildId) {
    await rest.put(
      Routes.applicationGuildCommands(config.appId, config.guildId),
      { body },
    );
    logger.info("Registered guild slash commands", {
      guildId: config.guildId,
      count: body.length,
    });
  } else {
    await rest.put(Routes.applicationCommands(config.appId), { body });
    logger.info("Registered global slash commands", { count: body.length });
  }
}

main().catch((err) => {
  process.stderr.write(
    `Failed to register slash commands: ${
      err instanceof Error ? err.message : String(err)
    }\n`,
  );
  process.exit(1);
});
