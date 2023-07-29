/**
 * Challenge catalog stored in a JSON file.
 *
 * A missing or unreadable file is replaced with the bundled seed list. Writes
 * are queued and go through a temp file + rename, so two !add commands at once
 * can't leave broken JSON behind.
 */
import { promises as fs } from "fs";
import * as path from "path";
import type { Logger } from "../logger";

export interface Challenge {
  title: string;
  url: string;
  addedBy: string;
  addedAt: string;
}

export interface AddInput {
  title: string;
  url: string;
  addedBy: string;
}

export type AddResult =
  | { status: "added"; challenge: Challenge }
  | { status: "duplicate"; challenge: Challenge };

export interface CatalogService {
  list(): Promise<Challenge[]>;
  random(): Promise<Challenge | undefined>;
  add(input: AddInput): Promise<AddResult>;
}

/** Key used to spot duplicate URLs: lowercase host, no hash or trailing slash. */
export function normalizeUrl(url: string): string {
  try {
    const u = new URL(url);
    u.hash = "";
    u.hostname = u.hostname.toLowerCase();
    let s = u.toString();
    if (s.endsWith("/")) s = s.slice(0, -1);
    return s;
  } catch {
    return url.trim();
  }
}

function isChallenge(value: unknown): value is Challenge {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return typeof v.title === "string" && typeof v.url === "string";
}

function coerceChallenges(parsed: unknown): Challenge[] {
  if (!Array.isArray(parsed)) return [];
  return parsed.filter(isChallenge).map((c) => ({
    title: c.title,
    url: c.url,
    addedBy: typeof c.addedBy === "string" ? c.addedBy : "seed",
    addedAt: typeof c.addedAt === "string" ? c.addedAt : new Date(0).toISOString(),
  }));
}

export interface CatalogOptions {
  catalogPath: string;
  seedPath?: string;
  logger?: Logger;
  now?: () => Date;
}

export function createCatalog(options: CatalogOptions): CatalogService {
  const { catalogPath, seedPath, logger } = options;
  const now = options.now ?? (() => new Date());

  let cache: Challenge[] | null = null;
  let writeChain: Promise<void> = Promise.resolve();

  async function readSeed(): Promise<Challenge[]> {
    if (!seedPath) return [];
    try {
      const raw = await fs.readFile(seedPath, "utf8");
      return coerceChallenges(JSON.parse(raw));
    } catch (err) {
      logger?.warn("Could not read seed file", {
        seedPath,
        error: (err as Error).message,
      });
      return [];
    }
  }

  async function persist(data: Challenge[]): Promise<void> {
    await fs.mkdir(path.dirname(catalogPath), { recursive: true });
    const tmp = `${catalogPath}.${process.pid}.${Date.now()}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(data, null, 2), "utf8");
    await fs.rename(tmp, catalogPath);
  }

  function enqueueWrite(data: Challenge[]): Promise<void> {
    const next = writeChain.then(() => persist(data));
    // A failed write shouldn't block the ones queued after it.
    writeChain = next.catch(() => undefined);
    return next;
  }

  async function load(): Promise<Challenge[]> {
    if (cache !== null) return cache;

    let raw: string;
    try {
      raw = await fs.readFile(catalogPath, "utf8");
    } catch {
      // First run: no catalog file yet.
      const seeded = await readSeed();
      cache = seeded;
      await enqueueWrite(seeded);
      logger?.info("Catalog initialized from seed", { count: seeded.length });
      return cache;
    }

    try {
      cache = coerceChallenges(JSON.parse(raw));
      return cache;
    } catch {
      logger?.warn("Catalog file was corrupt - recovering from seed", {
        catalogPath,
      });
      const seeded = await readSeed();
      cache = seeded;
      await enqueueWrite(seeded);
      return cache;
    }
  }

  return {
    async list(): Promise<Challenge[]> {
      return [...(await load())];
    },

    async random(): Promise<Challenge | undefined> {
      const data = await load();
      if (data.length === 0) return undefined;
      return data[Math.floor(Math.random() * data.length)];
    },

    async add(input: AddInput): Promise<AddResult> {
      const data = await load();
      const key = normalizeUrl(input.url);
      const existing = data.find((c) => normalizeUrl(c.url) === key);
      if (existing) {
        return { status: "duplicate", challenge: existing };
      }
      const challenge: Challenge = {
        title: input.title,
        url: input.url,
        addedBy: input.addedBy,
        addedAt: now().toISOString(),
      };
      const updated = [...data, challenge];
      cache = updated;
      await enqueueWrite(updated);
      return { status: "added", challenge };
    },
  };
}
