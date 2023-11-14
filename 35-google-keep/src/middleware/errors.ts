import type { NextFunction, Request, Response } from 'express';
import { ZodError } from 'zod';
import type { ErrorCode } from '../types';

export class HttpError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: ErrorCode,
    message: string,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = 'HttpError';
  }
}

export const notFound = (msg = 'Resource not found'): HttpError =>
  new HttpError(404, 'NOT_FOUND', msg);
export const unauthenticated = (msg = 'Authentication required'): HttpError =>
  new HttpError(401, 'UNAUTHENTICATED', msg);
export const validationError = (msg: string, details?: unknown): HttpError =>
  new HttpError(400, 'VALIDATION_ERROR', msg, details);

/** Catch-all Express error handler producing the consistent error envelope. */
export function errorHandler(
  err: unknown,
  req: Request,
  res: Response,
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  _next: NextFunction,
): void {
  if (err instanceof HttpError) {
    res.status(err.status).json({
      error: { code: err.code, message: err.message, details: err.details },
    });
    return;
  }
  if (err instanceof ZodError) {
    const first = err.errors[0];
    const field = first?.path.join('.') || 'body';
    res.status(400).json({
      error: {
        code: 'VALIDATION_ERROR',
        message: `${field}: ${first?.message ?? 'invalid input'}`,
        details: err.errors,
      },
    });
    return;
  }
  const message = err instanceof Error ? err.message : 'Unexpected error';
  req.log?.error?.({ err: message }, 'unhandled error');
  res.status(500).json({ error: { code: 'INTERNAL_ERROR', message: 'Internal server error' } });
}

/** Wrap an async route handler so thrown errors reach the error middleware. */
export function asyncHandler(
  fn: (req: Request, res: Response, next: NextFunction) => unknown | Promise<unknown>,
) {
  return (req: Request, res: Response, next: NextFunction): void => {
    Promise.resolve(fn(req, res, next)).catch(next);
  };
}
