import type { NextFunction, Request, RequestHandler, Response } from 'express';
import { ZodError, type ZodTypeAny, type z } from 'zod';
import { AppError } from './errors.js';
import type { AuthService } from './services/auth-service.js';
import type { BlockService } from './services/block-service.js';
import type { PageService } from './services/page-service.js';
import type { Config } from './config.js';

export interface Services {
  auth: AuthService;
  pages: PageService;
  blocks: BlockService;
  config: Config;
}

/** Wrap an async handler so thrown/rejected errors reach the error middleware. */
export function asyncHandler(
  fn: (req: Request, res: Response, next: NextFunction) => unknown | Promise<unknown>,
): RequestHandler {
  return (req, res, next) => {
    Promise.resolve(fn(req, res, next)).catch(next);
  };
}

/** Validate `data` against a schema, throwing a VALIDATION_ERROR envelope. */
export function parse<T extends ZodTypeAny>(schema: T, data: unknown): z.infer<T> {
  const result = schema.safeParse(data);
  if (!result.success) {
    throw AppError.validation('Request validation failed', flattenZod(result.error));
  }
  return result.data;
}

function flattenZod(err: ZodError): unknown[] {
  return err.issues.map((i) => ({ path: i.path.join('.'), message: i.message }));
}

function bearer(req: Request): string | undefined {
  const header = req.header('authorization') ?? '';
  const [scheme, token] = header.split(' ');
  return scheme?.toLowerCase() === 'bearer' ? token : undefined;
}

/** Authentication middleware: resolves Bearer token to `req.userId`. */
export function requireAuth(auth: AuthService): RequestHandler {
  return (req, _res, next) => {
    try {
      req.userId = auth.authenticate(bearer(req));
      next();
    } catch (err) {
      next(err);
    }
  };
}

export function getUserId(req: Request): string {
  if (!req.userId) throw AppError.unauthenticated();
  return req.userId;
}

/** Terminal error handler emitting the documented error envelope. */
export const errorHandler = (
  err: unknown,
  _req: Request,
  res: Response,
  _next: NextFunction,
): void => {
  if (err instanceof AppError) {
    res.status(err.status).json({
      error: { code: err.code, message: err.message, details: err.details },
    });
    return;
  }
  if (err instanceof ZodError) {
    res.status(400).json({
      error: {
        code: 'VALIDATION_ERROR',
        message: 'Request validation failed',
        details: flattenZod(err),
      },
    });
    return;
  }
  // eslint-disable-next-line no-console
  console.error('Unhandled error:', err);
  res.status(500).json({
    error: { code: 'INTERNAL', message: 'Internal server error', details: [] },
  });
};
