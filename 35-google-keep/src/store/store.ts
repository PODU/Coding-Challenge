import fs from 'node:fs';
import path from 'node:path';
import type { NoteRow, NoteLabelRow, User, Label } from '../types';

interface StoreData {
  users: User[];
  notes: NoteRow[];
  labels: Label[];
  noteLabels: NoteLabelRow[];
}

function emptyData(): StoreData {
  return { users: [], notes: [], labels: [], noteLabels: [] };
}

/**
 * In-memory data store, saved to a single JSON file. The path ':memory:'
 * keeps everything off disk, for tests.
 *
 * Saves write a temp file and rename it, one at a time, so the file is never
 * left half-written.
 */
export class Store {
  readonly data: StoreData;
  private readonly filePath: string | null;
  private writeQueued = false;

  constructor(databasePath: string) {
    this.filePath = databasePath === ':memory:' ? null : databasePath;
    this.data = emptyData();
    if (this.filePath) {
      this.load();
    }
  }

  private load(): void {
    if (!this.filePath) return;
    try {
      if (fs.existsSync(this.filePath)) {
        const raw = fs.readFileSync(this.filePath, 'utf8');
        if (raw.trim()) {
          const parsed = JSON.parse(raw) as Partial<StoreData>;
          this.data.users = parsed.users ?? [];
          this.data.notes = parsed.notes ?? [];
          this.data.labels = parsed.labels ?? [];
          this.data.noteLabels = parsed.noteLabels ?? [];
        }
      }
    } catch (err) {
      throw new Error(`Failed to load store from ${this.filePath}: ${(err as Error).message}`);
    }
  }

  /** Persist current state to disk atomically. No-op for in-memory stores. */
  persist(): void {
    if (!this.filePath) return;
    const dir = path.dirname(this.filePath);
    fs.mkdirSync(dir, { recursive: true });
    const tmp = `${this.filePath}.${process.pid}.tmp`;
    fs.writeFileSync(tmp, JSON.stringify(this.data, null, 2), 'utf8');
    fs.renameSync(tmp, this.filePath);
  }

  /**
   * Save on the next tick, so several changes in one tick cost one write.
   */
  schedulePersist(): void {
    if (!this.filePath || this.writeQueued) return;
    this.writeQueued = true;
    setImmediate(() => {
      this.writeQueued = false;
      try {
        this.persist();
      } catch {
        // Surfaced via STORAGE_ERROR on the request path; nothing to do here.
      }
    });
  }
}
