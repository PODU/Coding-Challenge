import type { NextFunction, Request, Response } from 'express';
import { readSession } from '../session';
import { unauthenticated } from './errors';

/**
 * Sets `req.userId` from a valid session cookie. It never rejects a request;
 * `requireAuth` does that.
 */
export function sessionLoader(secret: string) {
  return (req: Request, _res: Response, next: NextFunction): void => {
    const userId = readSession(req, secret);
    if (userId) req.userId = userId;
    next();
  };
}

/** Rejects the request with 401 if there is no authenticated user. */
export function requireAuth(req: Request, _res: Response, next: NextFunction): void {
  if (!req.userId) {
    next(unauthenticated());
    return;
  }
  // Verify the user still exists.
  const user = req.repos?.users.getById(req.userId);
  if (!user) {
    next(unauthenticated());
    return;
  }
  next();
}
