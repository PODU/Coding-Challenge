import request from 'supertest';
import type { Express } from 'express';
import { loadConfig } from '../../src/config';
import { Store } from '../../src/store/store';
import { makeApp, makeRepos, type Repos } from '../../src/app';
import type { GoogleAuthClient } from '../../src/auth/google';

export interface TestCtx {
  app: Express;
  repos: Repos;
  store: Store;
}

export function makeTestApp(opts: { google?: GoogleAuthClient | null; env?: Record<string, string> } = {}): TestCtx {
  const config = loadConfig({
    DATABASE_PATH: ':memory:',
    AUTH_LOCAL: '1',
    SESSION_SECRET: 'test-secret',
    ...opts.env,
  } as NodeJS.ProcessEnv);
  const store = new Store(':memory:');
  const { app, repos } = makeApp({
    config,
    store,
    logSilent: true,
    google: opts.google ?? null,
  });
  return { app, repos, store };
}

/** A bare in-memory store + repos for unit-testing the data layer. */
export function makeTestRepos(): { store: Store; repos: Repos } {
  const store = new Store(':memory:');
  return { store, repos: makeRepos(store) };
}

/** Log in via the local dev path and return a cookie-persisting supertest agent. */
export async function loginAgent(
  app: Express,
  username: string,
): Promise<ReturnType<typeof request.agent>> {
  const agent = request.agent(app);
  await agent.post('/api/auth/local').send({ username }).expect(200);
  return agent;
}
