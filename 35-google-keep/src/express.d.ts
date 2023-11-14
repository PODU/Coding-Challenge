import type { Repos } from './app';

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      userId?: string;
      requestId?: string;
      repos?: Repos;
      log?: {
        info: (obj: unknown, msg?: string) => void;
        error: (obj: unknown, msg?: string) => void;
      };
    }
  }
}

export {};
