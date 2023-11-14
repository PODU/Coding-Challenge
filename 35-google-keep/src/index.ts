import { loadConfig } from './config';
import { Store } from './store/store';
import { makeApp } from './app';

function main(): void {
  const config = loadConfig();
  const store = new Store(config.databasePath);
  const { app } = makeApp({ config, store });

  const server = app.listen(config.port, () => {
    // eslint-disable-next-line no-console
    console.log(
      JSON.stringify({
        level: 'info',
        msg: 'server.started',
        port: config.port,
        authLocal: config.authLocal,
        google: Boolean(config.googleClientId && config.googleClientSecret),
        db: config.databasePath,
      }),
    );
  });

  const shutdown = (signal: string): void => {
    // eslint-disable-next-line no-console
    console.log(JSON.stringify({ level: 'info', msg: 'server.shutdown', signal }));
    server.close(() => {
      try {
        store.persist();
      } catch {
        /* best effort */
      }
      process.exit(0);
    });
    // Force-exit if connections linger.
    setTimeout(() => process.exit(0), 5000).unref();
  };

  process.on('SIGINT', () => shutdown('SIGINT'));
  process.on('SIGTERM', () => shutdown('SIGTERM'));
}

main();
