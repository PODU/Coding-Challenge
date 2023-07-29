import type { Command } from "../context";
import { hello } from "./hello";
import { quote } from "./quote";
import { challenge } from "./challenge";
import { add } from "./add";
import { list } from "./list";

export const commands: Command[] = [hello, quote, challenge, add, list];

export const commandRegistry: ReadonlyMap<string, Command> = new Map(
  commands.map((c) => [c.name, c]),
);

/** Commands exposed as Discord slash commands ("hello" stays text-only). */
export const slashCommands: Command[] = [quote, challenge, add, list];

export { hello, quote, challenge, add, list };
