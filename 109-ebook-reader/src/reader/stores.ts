/** Persistence abstraction so the same logic runs in browser, Electron or tests. */
export interface Storage {
  get(key: string): string | null;
  set(key: string, value: string): void;
  remove(key: string): void;
}

/** In-memory Storage (used by tests and as a default fallback). */
export class MemoryStorage implements Storage {
  private readonly map = new Map<string, string>();
  get(key: string): string | null {
    return this.map.has(key) ? this.map.get(key)! : null;
  }
  set(key: string, value: string): void {
    this.map.set(key, value);
  }
  remove(key: string): void {
    this.map.delete(key);
  }
}

/** Content-anchored location shared by bookmarks and annotations. */
export interface AnchoredLocation {
  spineIndex: number;
  anchor?: string;
}

export interface Bookmark extends AnchoredLocation {
  id: string;
  label?: string;
  createdAt: number;
}

export interface Annotation extends AnchoredLocation {
  id: string;
  text: string;
  note?: string;
  color: string;
  createdAt: number;
}

export interface ReaderSettings {
  theme: 'light' | 'dark';
  fontFamily: string;
  fontSizePx: number;
  lineHeight: number;
}

export const DEFAULT_SETTINGS: ReaderSettings = {
  theme: 'light',
  fontFamily: 'serif',
  fontSizePx: 18,
  lineHeight: 1.6,
};

abstract class JsonStore<T> {
  protected readonly storage: Storage;
  protected readonly key: string;
  constructor(storage: Storage, key: string) {
    this.storage = storage;
    this.key = key;
  }
  protected read<R>(fallback: R): R {
    const raw = this.storage.get(this.key);
    if (raw == null) return fallback;
    try {
      return JSON.parse(raw) as R;
    } catch {
      return fallback;
    }
  }
  protected write(value: T): void {
    this.storage.set(this.key, JSON.stringify(value));
  }
}

export class BookmarkStore extends JsonStore<Bookmark[]> {
  constructor(storage: Storage, bookId: string) {
    super(storage, `epub:${bookId}:bookmarks`);
  }
  list(): Bookmark[] {
    return this.read<Bookmark[]>([]);
  }
  add(b: Omit<Bookmark, 'id' | 'createdAt'>): Bookmark {
    const bookmark: Bookmark = { ...b, id: makeId(), createdAt: Date.now() };
    const all = this.list();
    all.push(bookmark);
    this.write(all);
    return bookmark;
  }
  remove(id: string): void {
    this.write(this.list().filter((b) => b.id !== id));
  }
}

export class AnnotationStore extends JsonStore<Annotation[]> {
  constructor(storage: Storage, bookId: string) {
    super(storage, `epub:${bookId}:annotations`);
  }
  list(): Annotation[] {
    return this.read<Annotation[]>([]);
  }
  add(a: Omit<Annotation, 'id' | 'createdAt'>): Annotation {
    const annotation: Annotation = { ...a, id: makeId(), createdAt: Date.now() };
    const all = this.list();
    all.push(annotation);
    this.write(all);
    return annotation;
  }
  remove(id: string): void {
    this.write(this.list().filter((a) => a.id !== id));
  }
}

export class SettingsStore extends JsonStore<ReaderSettings> {
  constructor(storage: Storage, key = 'epub:settings') {
    super(storage, key);
  }
  get(): ReaderSettings {
    return { ...DEFAULT_SETTINGS, ...this.read<Partial<ReaderSettings>>({}) };
  }
  save(settings: Partial<ReaderSettings>): ReaderSettings {
    const merged = { ...this.get(), ...settings };
    this.write(merged);
    return merged;
  }
}

let counter = 0;
function makeId(): string {
  counter += 1;
  return `${Date.now().toString(36)}-${counter.toString(36)}`;
}
