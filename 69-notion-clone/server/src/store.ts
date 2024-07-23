/**
 * Data layer.
 *
 * A plain JS store rather than SQLite: better-sqlite3 is a native addon and
 * needs a C++ toolchain to install, which we can't count on (Windows in
 * particular).
 *
 * Everything lives in Maps. Unless the path is ':memory:', a JSON snapshot is
 * written to disk. `transaction()` makes multi-row changes such as moves,
 * duplicates and subtree deletes all-or-nothing.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import type { Block, Page } from '../../shared/types.js';

export interface UserRow {
  id: string;
  email: string;
  passwordHash: string;
  createdAt: string;
}

interface Snapshot {
  users: UserRow[];
  pages: Page[];
  blocks: Block[];
}

export interface Store {
  // users
  insertUser(user: UserRow): void;
  getUserById(id: string): UserRow | undefined;
  getUserByEmail(email: string): UserRow | undefined;

  // pages
  insertPage(page: Page): void;
  getPage(id: string): Page | undefined;
  listPagesByUser(userId: string): Page[];
  updatePage(page: Page): void;
  deletePage(id: string): void;

  // blocks
  insertBlock(block: Block): void;
  getBlock(id: string): Block | undefined;
  listBlocksByPage(pageId: string): Block[];
  updateBlock(block: Block): void;
  deleteBlock(id: string): void;
  deleteBlocksByPage(pageId: string): void;

  /** Run `fn` atomically; on throw the store is rolled back to its prior state. */
  transaction<T>(fn: () => T): T;

  /** Persist to disk if file-backed (no-op for in-memory). */
  flush(): void;
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

export class MemoryStore implements Store {
  private users = new Map<string, UserRow>();
  private pages = new Map<string, Page>();
  private blocks = new Map<string, Block>();
  private readonly filePath: string | null;
  private inTransaction = false;
  private dirty = false;

  constructor(dataPath = ':memory:') {
    this.filePath = dataPath === ':memory:' ? null : dataPath;
    if (this.filePath && existsSync(this.filePath)) {
      this.load();
    }
  }

  private load(): void {
    if (!this.filePath) return;
    const raw = readFileSync(this.filePath, 'utf8');
    const snap = JSON.parse(raw) as Snapshot;
    this.users = new Map((snap.users ?? []).map((u) => [u.id, u]));
    this.pages = new Map((snap.pages ?? []).map((p) => [p.id, p]));
    this.blocks = new Map((snap.blocks ?? []).map((b) => [b.id, b]));
  }

  flush(): void {
    if (!this.filePath || this.inTransaction) return;
    if (!this.dirty) return;
    const snap: Snapshot = {
      users: [...this.users.values()],
      pages: [...this.pages.values()],
      blocks: [...this.blocks.values()],
    };
    mkdirSync(dirname(this.filePath), { recursive: true });
    writeFileSync(this.filePath, JSON.stringify(snap), 'utf8');
    this.dirty = false;
  }

  private touch(): void {
    this.dirty = true;
    if (!this.inTransaction) this.flush();
  }

  transaction<T>(fn: () => T): T {
    if (this.inTransaction) return fn(); // nested calls join the outer txn
    const backup: Snapshot = {
      users: [...this.users.values()].map(clone),
      pages: [...this.pages.values()].map(clone),
      blocks: [...this.blocks.values()].map(clone),
    };
    this.inTransaction = true;
    try {
      const result = fn();
      this.inTransaction = false;
      this.flush();
      return result;
    } catch (err) {
      this.users = new Map(backup.users.map((u) => [u.id, u]));
      this.pages = new Map(backup.pages.map((p) => [p.id, p]));
      this.blocks = new Map(backup.blocks.map((b) => [b.id, b]));
      this.inTransaction = false;
      throw err;
    }
  }

  insertUser(user: UserRow): void {
    this.users.set(user.id, clone(user));
    this.touch();
  }
  getUserById(id: string): UserRow | undefined {
    const u = this.users.get(id);
    return u ? clone(u) : undefined;
  }
  getUserByEmail(email: string): UserRow | undefined {
    const key = email.toLowerCase();
    for (const u of this.users.values()) {
      if (u.email.toLowerCase() === key) return clone(u);
    }
    return undefined;
  }

  insertPage(page: Page): void {
    this.pages.set(page.id, clone(page));
    this.touch();
  }
  getPage(id: string): Page | undefined {
    const p = this.pages.get(id);
    return p ? clone(p) : undefined;
  }
  listPagesByUser(userId: string): Page[] {
    return [...this.pages.values()]
      .filter((p) => p.userId === userId)
      .map(clone);
  }
  updatePage(page: Page): void {
    this.pages.set(page.id, clone(page));
    this.touch();
  }
  deletePage(id: string): void {
    this.pages.delete(id);
    this.touch();
  }

  insertBlock(block: Block): void {
    this.blocks.set(block.id, clone(block));
    this.touch();
  }
  getBlock(id: string): Block | undefined {
    const b = this.blocks.get(id);
    return b ? clone(b) : undefined;
  }
  listBlocksByPage(pageId: string): Block[] {
    return [...this.blocks.values()]
      .filter((b) => b.pageId === pageId)
      .map(clone);
  }
  updateBlock(block: Block): void {
    this.blocks.set(block.id, clone(block));
    this.touch();
  }
  deleteBlock(id: string): void {
    this.blocks.delete(id);
    this.touch();
  }
  deleteBlocksByPage(pageId: string): void {
    for (const b of [...this.blocks.values()]) {
      if (b.pageId === pageId) this.blocks.delete(b.id);
    }
    this.touch();
  }
}

export function createStore(dataPath?: string): Store {
  return new MemoryStore(dataPath);
}
