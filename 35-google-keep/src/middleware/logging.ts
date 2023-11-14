import { randomUUID } from 'node:crypto';
import type { NextFunction, Request, Response } from 'express';

/**
 * Logs one JSON line per request and gives each request an id and a `log`
 * helper. LOG_SILENT turns it off for the tests.
 */
export function requestLogger(silent = false) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const id = randomUUID();
    req.requestId = id;
    const emit = (level: 'info' | 'error', obj: unknown, msg?: string): void => {
      if (silent) return;
      // eslint-disable-next-line no-console
      console[level === 'error' ? 'error' : 'log'](
        JSON.stringify({ level, time: new Date().toISOString(), reqId: id, msg, ...(obj as object) }),
      );
    };
    req.log = {
      info: (obj, msg) => emit('info', obj, msg),
      error: (obj, msg) => emit('error', obj, msg),
    };
    const start = Date.now();
    res.on('finish', () => {
      req.log?.info(
        { method: req.method, path: req.path, status: res.statusCode, ms: Date.now() - start },
        'request',
      );
    });
    next();
  };
}
