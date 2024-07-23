import express, { type Express } from 'express';
import { existsSync } from 'node:fs';
import { join, resolve } from 'node:path';
import type { Config } from './config.js';
import { loadConfig } from './config.js';
import { createStore, type Store } from './store.js';
import { AuthService } from './services/auth-service.js';
import { PageService } from './services/page-service.js';
import { BlockService } from './services/block-service.js';
import { errorHandler, type Services } from './http.js';
import { authRouter } from './routes/auth.js';
import { pagesRouter } from './routes/pages.js';
import { blocksRouter, pageBlocksRouter } from './routes/blocks.js';
import { aiRouter } from './routes/ai.js';

export interface AppBundle {
  app: Express;
  services: Services;
  store: Store;
}

export function buildServices(store: Store, config: Config): Services {
  return {
    auth: new AuthService(store, config),
    pages: new PageService(store),
    blocks: new BlockService(store),
    config,
  };
}

export interface AppOptions {
  serveStatic?: boolean;
}

export function createApp(
  store: Store,
  config: Config,
  options: AppOptions = {},
): AppBundle {
  const services = buildServices(store, config);
  const app = express();
  app.use(express.json({ limit: '5mb' }));

  app.get('/api/health', (_req, res) => res.json({ status: 'ok' }));
  app.use('/api/auth', authRouter(services));
  app.use('/api/pages', pagesRouter(services));
  app.use('/api/pages', pageBlocksRouter(services));
  app.use('/api/blocks', blocksRouter(services));
  app.use('/api/ai', aiRouter(services));

  if (options.serveStatic) {
    const webDir = resolve(process.cwd(), 'web', 'dist');
    if (existsSync(webDir)) {
      app.use(express.static(webDir));
      app.get(/^(?!\/api).*/, (_req, res) => {
        res.sendFile(join(webDir, 'index.html'));
      });
    }
  }

  app.use(errorHandler);
  return { app, services, store };
}

/** An app with its own in-memory store, for tests. */
export function makeTestApp(configOverrides: Partial<Config> = {}): AppBundle {
  const config: Config = {
    ...loadConfig({}),
    jwtSecret: 'test-secret',
    dataPath: ':memory:',
    ...configOverrides,
  };
  const store = createStore(':memory:');
  return createApp(store, config, { serveStatic: false });
}
