import express, { type Express, type NextFunction, type Request, type Response } from 'express';
import path from 'node:path';
import { config } from '../config';
import { Executor } from '../exec/executor';
import { QueueFullError } from '../exec/queue';
import { gradeSubmission } from '../problems/grader';
import { ProblemStore, toPublicProblem } from '../problems/store';
import { defaultRegistry, type RunnerRegistry } from '../runners/registry';
import type { LanguageInfo } from '../types';
import { rateLimiter } from './rateLimit';
import { isSizeViolation, runRequestSchema, submitRequestSchema } from './schemas';

export interface AppDeps {
  registry?: RunnerRegistry;
  executor?: Executor;
  problems?: ProblemStore;
  /** Serve the static editor SPA from web/. Disabled in tests. */
  serveWeb?: boolean;
}

export interface BuiltApp {
  app: Express;
  registry: RunnerRegistry;
  executor: Executor;
  problems: ProblemStore;
}

/**
 * Build the Express app. All dependencies are injectable so tests can supply a
 * fake sandbox / in-memory problem store without spawning real processes.
 */
export function createApp(deps: AppDeps = {}): BuiltApp {
  const registry = deps.registry ?? defaultRegistry();
  const executor = deps.executor ?? new Executor(registry);
  const problems = deps.problems ?? new ProblemStore();

  const app = express();
  app.disable('x-powered-by');
  app.set('trust proxy', true);

  // Body size cap -> oversized payloads rejected with 413 before any spawn.
  app.use(
    express.json({
      limit: config.maxSourceBytes + config.maxStdinBytes + 4096,
    }),
  );

  // Lock CORS to a configured origin (default: same-origin only).
  app.use((req: Request, res: Response, next: NextFunction) => {
    const origin = process.env.CORS_ORIGIN;
    if (origin) {
      res.setHeader('Access-Control-Allow-Origin', origin);
      res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
      res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS');
      if (req.method === 'OPTIONS') {
        res.sendStatus(204);
        return;
      }
    }
    next();
  });

  const limiter = rateLimiter(config.rateLimitMax, config.rateLimitWindowMs);

  let cachedLanguages: LanguageInfo[] | null = null;

  app.get('/api/health', (_req, res) => {
    res.json({ status: 'ok', version: config.version });
  });

  app.get('/api/languages', async (_req, res, next) => {
    try {
      if (!cachedLanguages) cachedLanguages = await registry.languageInfos();
      res.json({ languages: cachedLanguages });
    } catch (err) {
      next(err);
    }
  });

  app.post('/api/run', limiter, async (req, res, next) => {
    const parsed = runRequestSchema.safeParse(req.body);
    if (!parsed.success) {
      const status = isSizeViolation(parsed.error) ? 413 : 400;
      res.status(status).json({
        status: 'error',
        error: {
          type: 'validation',
          message: parsed.error.issues.map((i) => i.message).join('; '),
          line: null,
          traceback: null,
        },
      });
      return;
    }
    try {
      const result = await executor.run(parsed.data);
      res.json(result);
    } catch (err) {
      if (err instanceof QueueFullError) {
        res.status(429).json({
          status: 'error',
          error: { type: 'validation', message: err.message, line: null, traceback: null },
        });
        return;
      }
      next(err);
    }
  });

  app.get('/api/problems', (_req, res) => {
    res.json({ problems: problems.list() });
  });

  app.get('/api/problems/:id', (req, res) => {
    const problem = problems.get(req.params.id ?? '');
    if (!problem) {
      res.status(404).json({
        status: 'error',
        error: { type: 'validation', message: 'Problem not found', line: null, traceback: null },
      });
      return;
    }
    res.json(toPublicProblem(problem));
  });

  app.post('/api/problems/:id/submit', limiter, async (req, res, next) => {
    const problem = problems.get(req.params.id ?? '');
    if (!problem) {
      res.status(404).json({
        status: 'error',
        error: { type: 'validation', message: 'Problem not found', line: null, traceback: null },
      });
      return;
    }
    const parsed = submitRequestSchema.safeParse(req.body);
    if (!parsed.success) {
      const status = isSizeViolation(parsed.error) ? 413 : 400;
      res.status(status).json({
        status: 'error',
        error: {
          type: 'validation',
          message: parsed.error.issues.map((i) => i.message).join('; '),
          line: null,
          traceback: null,
        },
      });
      return;
    }
    if (!problem.languages.includes(parsed.data.language)) {
      res.status(400).json({
        status: 'error',
        error: {
          type: 'validation',
          message: `Language ${parsed.data.language} not allowed for this problem`,
          line: null,
          traceback: null,
        },
      });
      return;
    }
    try {
      const result = await gradeSubmission(executor, problem, parsed.data.language, parsed.data.source);
      res.json(result);
    } catch (err) {
      if (err instanceof QueueFullError) {
        res.status(429).json({
          status: 'error',
          error: { type: 'validation', message: err.message, line: null, traceback: null },
        });
        return;
      }
      next(err);
    }
  });

  if (deps.serveWeb) {
    // Compiled location is dist/src/api -> project root is three levels up.
    const webDir = path.resolve(__dirname, '../../../web');
    app.use(express.static(webDir));
    app.get('/', (_req, res) => res.sendFile(path.join(webDir, 'index.html')));
  }

  app.use((err: unknown, _req: Request, res: Response, next: NextFunction) => {
    if (err && typeof err === 'object' && 'type' in err && (err as { type?: string }).type === 'entity.too.large') {
      res.status(413).json({
        status: 'error',
        error: { type: 'validation', message: 'Payload too large', line: null, traceback: null },
      });
      return;
    }
    if (err instanceof SyntaxError && 'body' in err) {
      res.status(400).json({
        status: 'error',
        error: { type: 'validation', message: 'Malformed JSON body', line: null, traceback: null },
      });
      return;
    }
    next(err);
  });

  app.use((err: unknown, _req: Request, res: Response, _next: NextFunction) => {
    const message = err instanceof Error ? err.message : 'Internal server error';
    res.status(500).json({
      status: 'internal_error',
      error: { type: 'internal', message, line: null, traceback: null },
    });
  });

  return { app, registry, executor, problems };
}
