import { mkdir, readFile, writeFile, rename } from 'node:fs/promises';
import path from 'node:path';
import type { Recording } from '../../../shared/types';

export interface ListOptions {
  limit?: number;
  cursor?: string;
}

export interface ListResult {
  items: Recording[];
  nextCursor?: string;
}

export interface RecordingsRepo {
  create(rec: Recording): Promise<Recording>;
  getById(id: string): Promise<Recording | null>;
  getByShareId(shareId: string): Promise<Recording | null>;
  list(opts?: ListOptions): Promise<ListResult>;
  delete(id: string): Promise<boolean>;
}

const DEFAULT_LIMIT = 20;
const MAX_LIMIT = 100;

/** Also used as the pagination cursor; the id breaks timestamp ties. */
function sortKey(r: Recording): string {
  return `${r.createdAt}|${r.id}`;
}

function paginate(all: Recording[], opts?: ListOptions): ListResult {
  const limit = Math.min(Math.max(1, opts?.limit ?? DEFAULT_LIMIT), MAX_LIMIT);
  const sorted = [...all].sort((a, b) => (sortKey(a) < sortKey(b) ? 1 : -1));
  let start = 0;
  if (opts?.cursor) {
    const idx = sorted.findIndex((r) => sortKey(r) === opts.cursor);
    start = idx >= 0 ? idx + 1 : sorted.length;
  }
  const items = sorted.slice(start, start + limit);
  const result: ListResult = { items };
  if (start + limit < sorted.length && items.length > 0) {
    result.nextCursor = sortKey(items[items.length - 1] as Recording);
  }
  return result;
}

/** Used directly by tests, and as the base of the JSON-file repo. */
export class InMemoryRecordingsRepo implements RecordingsRepo {
  protected readonly byId = new Map<string, Recording>();

  async create(rec: Recording): Promise<Recording> {
    this.byId.set(rec.id, rec);
    await this.persist();
    return rec;
  }

  async getById(id: string): Promise<Recording | null> {
    return this.byId.get(id) ?? null;
  }

  async getByShareId(shareId: string): Promise<Recording | null> {
    for (const rec of this.byId.values()) {
      if (rec.shareId === shareId) {
        return rec;
      }
    }
    return null;
  }

  async list(opts?: ListOptions): Promise<ListResult> {
    return paginate([...this.byId.values()], opts);
  }

  async delete(id: string): Promise<boolean> {
    const existed = this.byId.delete(id);
    if (existed) {
      await this.persist();
    }
    return existed;
  }

  protected async persist(): Promise<void> {}
}

/** Keeps every recording in one JSON file, rewritten via temp file + rename. */
export class JsonRecordingsRepo extends InMemoryRecordingsRepo {
  private readonly file: string;
  private loaded = false;

  constructor(file: string) {
    super();
    this.file = file;
  }

  async load(): Promise<void> {
    if (this.loaded) {
      return;
    }
    try {
      const raw = await readFile(this.file, 'utf8');
      const rows = JSON.parse(raw) as Recording[];
      for (const r of rows) {
        this.byId.set(r.id, r);
      }
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== 'ENOENT') {
        throw err;
      }
    }
    this.loaded = true;
  }

  protected override async persist(): Promise<void> {
    await mkdir(path.dirname(this.file), { recursive: true });
    const tmp = `${this.file}.tmp`;
    await writeFile(tmp, JSON.stringify([...this.byId.values()], null, 2), 'utf8');
    await rename(tmp, this.file);
  }
}
