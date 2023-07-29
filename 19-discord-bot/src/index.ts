/** Bot entry point. */
import * as path from "path";
import { loadConfig } from "./config";
import { createLogger } from "./logger";
import { createClient, wireEvents } from "./client";
import type { Deps } from "./context";
import { createQuotesService } from "./services/quotes";
import { createHttpService } from "./services/http";
import { createCatalog } from "./services/catalog";

async function main(): Promise<void> {
  const config = loadConfig();
  const logger = createLogger(config.logLevel);

  const deps: Deps = {
    quotes: createQuotesService(config.quotesApiUrl),
    http: createHttpService(),
    catalog: createCatalog({
      catalogPath: config.catalogPath,
      seedPath: path.resolve(__dirname, "..", "seed", "challenges.seed.json"),
      logger,
    }),
  };

  const client = createClient();
  wireEvents({ client, config, deps, logger });

  // Log stray errors instead of letting them take the bot down.
  process.on("unhandledRejection", (reason) => {
    logger.error("Unhandled promise rejection", {
      reason: reason instanceof Error ? reason.message : String(reason),
    });
  });
  process.on("uncaughtException", (err) => {
    logger.error("Uncaught exception", { error: err.message });
  });

  const shutdown = (signal: string): void => {
    logger.info("Shutting down", { signal });
    client
      .destroy()
      .catch(() => undefined)
      .finally(() => process.exit(0));
  };
  process.on("SIGINT", () => shutdown("SIGINT"));
  process.on("SIGTERM", () => shutdown("SIGTERM"));

  await client.login(config.token);
}

main().catch((err) => {
  // Startup failed (missing token, bad login, ...). Exit non-zero so the
  // supervisor notices.
  process.stderr.write(
    JSON.stringify({
      ts: new Date().toISOString(),
      level: "error",
      msg: "Fatal startup error",
      error: err instanceof Error ? err.message : String(err),
    }) + "\n",
  );
  process.exit(1);
});
