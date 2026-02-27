import type { NextFunction, Request, Response } from 'express';

interface Bucket {
  count: number;
  resetAt: number;
}

/**
 * Minimal in-memory per-IP fixed-window rate limiter. For a single process /
 * dev this is sufficient; a horizontally-scaled deployment should use a shared
 * store (Redis) instead.
 */
export function rateLimiter(maxPerWindow: number, windowMs: number) {
  const buckets = new Map<string, Bucket>();

  return function rateLimit(req: Request, res: Response, next: NextFunction): void {
    const key = req.ip ?? req.socket.remoteAddress ?? 'unknown';
    const now = Date.now();
    let bucket = buckets.get(key);

    if (!bucket || now >= bucket.resetAt) {
      bucket = { count: 0, resetAt: now + windowMs };
      buckets.set(key, bucket);
    }

    bucket.count += 1;
    if (bucket.count > maxPerWindow) {
      const retryAfter = Math.ceil((bucket.resetAt - now) / 1000);
      res.setHeader('Retry-After', String(retryAfter));
      res.status(429).json({
        status: 'error',
        error: { type: 'validation', message: 'Rate limit exceeded. Try again shortly.', line: null, traceback: null },
      });
      return;
    }

    next();
  };
}
