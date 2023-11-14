import path from 'node:path';

export interface Config {
  port: number;
  databasePath: string; // file path or ':memory:'
  sessionSecret: string;
  authLocal: boolean;
  googleClientId: string | null;
  googleClientSecret: string | null;
  publicDir: string;
}

function bool(v: string | undefined, fallback = false): boolean {
  if (v === undefined) return fallback;
  return v === '1' || v.toLowerCase() === 'true' || v.toLowerCase() === 'yes';
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  return {
    port: Number(env.PORT ?? 3000),
    databasePath: env.DATABASE_PATH ?? path.join(process.cwd(), 'data', 'keep.json'),
    sessionSecret: env.SESSION_SECRET ?? 'dev-insecure-secret-change-me',
    authLocal: bool(env.AUTH_LOCAL, true),
    googleClientId: env.GOOGLE_CLIENT_ID ?? null,
    googleClientSecret: env.GOOGLE_CLIENT_SECRET ?? null,
    publicDir: env.PUBLIC_DIR ?? path.join(__dirname, '..', 'public'),
  };
}
