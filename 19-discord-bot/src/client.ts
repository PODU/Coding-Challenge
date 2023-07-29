/**
 * Creates the discord.js client and hooks its events up to the router and the
 * slash-command handler.
 */
import {
  Client,
  Events,
  GatewayIntentBits,
  Partials,
  type Message,
  type Interaction,
} from "discord.js";
import type { Config } from "./config";
import type { Deps } from "./context";
import type { Logger } from "./logger";
import { dispatchMessage } from "./router";
import { dispatchInteraction, type SlashInteraction } from "./slash";

export function createClient(): Client {
  return new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMessages,
      GatewayIntentBits.MessageContent,
    ],
    partials: [Partials.Channel],
  });
}

export interface WireOptions {
  client: Client;
  config: Config;
  deps: Deps;
  logger: Logger;
}

export function wireEvents(opts: WireOptions): void {
  const { client, config, deps, logger } = opts;

  client.once(Events.ClientReady, (c) => {
    logger.info("Gateway ready", { tag: c.user.tag, id: c.user.id });
  });

  client.on(Events.MessageCreate, (message: Message) => {
    // Without the privileged MESSAGE CONTENT intent, Discord delivers user
    // messages with an empty body. Warn so the cause is obvious.
    if (
      !message.author.bot &&
      message.content === "" &&
      message.embeds.length === 0 &&
      message.attachments.size === 0
    ) {
      logger.warn(
        "Received an empty message body - the MESSAGE CONTENT INTENT may be " +
          "disabled in the Discord Developer Portal.",
      );
    }

    void dispatchMessage(
      {
        content: message.content,
        authorIsBot: message.author.bot,
        user: {
          id: message.author.id,
          username: message.author.username,
        },
        reply: async (text: string) => {
          await message.reply(text);
        },
      },
      { prefix: config.prefix, deps, logger },
    );
  });

  client.on(Events.InteractionCreate, (interaction: Interaction) => {
    if (!interaction.isChatInputCommand()) return;

    const adapter: SlashInteraction = {
      commandName: interaction.commandName,
      user: {
        id: interaction.user.id,
        username: interaction.user.username,
      },
      getStringOption: (name: string) =>
        interaction.options.getString(name) ?? null,
      reply: async (text: string) => {
        if (interaction.deferred || interaction.replied) {
          await interaction.followUp(text);
        } else {
          await interaction.reply(text);
        }
      },
    };

    void dispatchInteraction(adapter, { deps, logger });
  });

  client.on(Events.ShardDisconnect, (event, shardId) => {
    logger.warn("Shard disconnected", { shardId, code: event.code });
  });

  client.on(Events.ShardResume, (shardId, replayed) => {
    logger.info("Shard resumed", { shardId, replayed });
  });

  client.on(Events.Error, (err) => {
    logger.error("Client error", { error: err.message });
  });
}
