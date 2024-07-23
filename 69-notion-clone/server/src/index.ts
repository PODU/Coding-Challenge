import type { Server } from 'node:http';
import { loadConfig } from './config.js';
import { createStore } from './store.js';
import { createApp } from './app.js';

function main(): void {
  const config = loadConfig();
  const store = createStore(config.dataPath);
  const { app } = createApp(store, config, { serveStatic: true });

  const server: Server = app.listen(config.port, () => {
    // eslint-disable-next-line no-console
    console.log(
      `notion-clone API listening on http://localhost:${config.port} ` +
        `(data: ${config.dataPath})`,
    );
  });

  const shutdown = (signal: string): void => {
    // eslint-disable-next-line no-console
    console.log(`\n${signal} received, shutting down gracefully...`);
    server.close(() => {
      store.flush();
      process.exit(0);
    });
    setTimeout(() => process.exit(1), 10_000).unref();
  };

  process.on('SIGINT', () => shutdown('SIGINT'));
  process.on('SIGTERM', () => shutdown('SIGTERM'));
}

main();
