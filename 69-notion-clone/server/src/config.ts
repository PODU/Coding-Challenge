/** Runtime configuration sourced from environment variables. */

export interface Config {
  port: number;
  jwtSecret: string;
  jwtExpirySeconds: number;
  /** Path to the JSON data file, or ':memory:' for an ephemeral store. */
  dataPath: string;
  aiEnabled: boolean;
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  return {
    port: Number(env.PORT ?? 3000),
    jwtSecret: env.JWT_SECRET ?? 'dev-insecure-secret-change-me',
    jwtExpirySeconds: Number(env.JWT_EXPIRY_SECONDS ?? 60 * 60 * 24 * 7),
    dataPath: env.DATA_PATH ?? 'data/notion.json',
    aiEnabled: env.AI_ENABLED === 'true',
  };
}
