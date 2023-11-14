import path from 'node:path';
import fs from 'node:fs';
import express, { type Express, type NextFunction, type Request, type Response } from 'express';
import type { Config } from './config';
import { Store } from './store/store';
import { UserRepo } from './repos/userRepo';
import { LabelRepo } from './repos/labelRepo';
import { NoteRepo } from './repos/noteRepo';
import { requestLogger } from './middleware/logging';
import { sessionLoader } from './middleware/auth';
import { errorHandler, notFound } from './middleware/errors';
import { notesRouter } from './routes/notes';
import { labelsRouter } from './routes/labels';
import { authRouter } from './routes/auth';
import { createGoogleAuthClient, type GoogleAuthClient } from './auth/google';

export interface Repos {
  users: UserRepo;
  labels: LabelRepo;
  notes: NoteRepo;
}

export interface MakeAppDeps {
  config: Config;
  store?: Store;
  /** Inject a fake Google client in tests; omit to derive from config. */
  google?: GoogleAuthClient | null;
  logSilent?: boolean;
}

export interface MadeApp {
  app: Express;
  repos: Repos;
  store: Store;
}

export function makeRepos(store: Store): Repos {
  const users = new UserRepo(store);
  const labels = new LabelRepo(store);
  const notes = new NoteRepo(store, labels);
  return { users, labels, notes };
}

export function makeApp(deps: MakeAppDeps): MadeApp {
  const { config } = deps;
  const store = deps.store ?? new Store(config.databasePath);
  const repos = makeRepos(store);

  let google: GoogleAuthClient | null = deps.google ?? null;
  if (google === null && deps.google === undefined && config.googleClientId && config.googleClientSecret) {
    google = createGoogleAuthClient(config.googleClientId, config.googleClientSecret);
  }

  const app = express();
  app.disable('x-powered-by');
  app.use(express.json({ limit: '1mb' }));
  app.use(requestLogger(deps.logSilent ?? false));

  // Attach repos + resolve session on every request.
  app.use((req: Request, _res: Response, next: NextFunction) => {
    req.repos = repos;
    next();
  });
  app.use(sessionLoader(config.sessionSecret));

  app.get('/api/health', (_req, res) => res.json({ ok: true }));
  app.use('/api/auth', authRouter({ config, google }));
  app.use('/api/notes', notesRouter());
  app.use('/api/labels', labelsRouter());

  // Unknown /api routes get a JSON 404 rather than the SPA page.
  app.use('/api', (_req, _res, next) => next(notFound('Unknown API endpoint')));

  if (fs.existsSync(config.publicDir)) {
    app.use(express.static(config.publicDir));
    app.get('*', (req, res, next) => {
      if (req.path.startsWith('/api/')) return next();
      res.sendFile(path.join(config.publicDir, 'index.html'));
    });
  }

  app.use(errorHandler);
  return { app, repos, store };
}
