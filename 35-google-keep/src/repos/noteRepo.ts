import { randomUUID } from 'node:crypto';
import type { Store } from '../store/store';
import type { LabelRepo } from './labelRepo';
import type { Note, NoteColor, NoteRow } from '../types';

export interface CreateNoteInput {
  title?: string;
  content?: string;
  color?: NoteColor;
  pinned?: boolean;
  archived?: boolean;
  labels?: string[];
}

export interface UpdateNoteInput {
  title?: string;
  content?: string;
  color?: NoteColor;
  pinned?: boolean;
  archived?: boolean;
  position?: number;
  labels?: string[];
}

export interface ListFilter {
  q?: string;
  label?: string;
  archived?: boolean; // undefined => only non-archived
  trashed?: boolean; // undefined => only non-trashed
  pinned?: boolean; // undefined => any
  sort?: 'default' | 'created' | 'updated';
  limit?: number;
  offset?: number;
}

export class NoteRepo {
  constructor(
    private readonly store: Store,
    private readonly labels: LabelRepo,
  ) {}

  private labelNamesFor(noteId: string): string[] {
    const ids = this.store.data.noteLabels
      .filter((nl) => nl.noteId === noteId)
      .map((nl) => nl.labelId);
    return this.store.data.labels
      .filter((l) => ids.includes(l.id))
      .map((l) => l.name)
      .sort((a, b) => a.localeCompare(b));
  }

  private toDto(row: NoteRow): Note {
    return {
      id: row.id,
      title: row.title,
      content: row.content,
      color: row.color,
      pinned: row.pinned,
      archived: row.archived,
      trashed: row.trashed,
      position: row.position,
      labels: this.labelNamesFor(row.id),
      createdAt: row.createdAt,
      updatedAt: row.updatedAt,
    };
  }

  private setLabels(userId: string, noteId: string, names: string[]): void {
    this.store.data.noteLabels = this.store.data.noteLabels.filter((nl) => nl.noteId !== noteId);
    const seen = new Set<string>();
    for (const raw of names) {
      const name = raw.trim();
      if (!name || seen.has(name)) continue;
      seen.add(name);
      const label = this.labels.getOrCreate(userId, name);
      this.store.data.noteLabels.push({ noteId, labelId: label.id });
    }
  }

  private findRow(userId: string, id: string): NoteRow | undefined {
    return this.store.data.notes.find((n) => n.id === id && n.userId === userId);
  }

  create(userId: string, input: CreateNoteInput): Note {
    const now = new Date().toISOString();
    const maxPosition = this.store.data.notes
      .filter((n) => n.userId === userId)
      .reduce((max, n) => Math.max(max, n.position), 0);
    const row: NoteRow = {
      id: randomUUID(),
      userId,
      title: input.title ?? '',
      content: input.content ?? '',
      color: input.color ?? 'default',
      pinned: input.pinned ?? false,
      archived: input.archived ?? false,
      trashed: false,
      position: maxPosition + 1,
      createdAt: now,
      updatedAt: now,
    };
    this.store.data.notes.push(row);
    if (input.labels && input.labels.length) {
      this.setLabels(userId, row.id, input.labels);
    }
    this.store.schedulePersist();
    return this.toDto(row);
  }

  get(userId: string, id: string): Note | undefined {
    const row = this.findRow(userId, id);
    return row ? this.toDto(row) : undefined;
  }

  list(userId: string, filter: ListFilter = {}): Note[] {
    let rows = this.store.data.notes.filter((n) => n.userId === userId);

    rows = rows.filter((n) => n.trashed === (filter.trashed ?? false));
    if (!filter.trashed) {
      rows = rows.filter((n) => n.archived === (filter.archived ?? false));
    }
    if (filter.pinned !== undefined) {
      rows = rows.filter((n) => n.pinned === filter.pinned);
    }
    if (filter.q) {
      const q = filter.q.toLowerCase();
      rows = rows.filter(
        (n) => n.title.toLowerCase().includes(q) || n.content.toLowerCase().includes(q),
      );
    }
    if (filter.label) {
      const label = this.store.data.labels.find(
        (l) => l.userId === userId && l.name === filter.label,
      );
      if (!label) return [];
      const noteIds = new Set(
        this.store.data.noteLabels.filter((nl) => nl.labelId === label.id).map((nl) => nl.noteId),
      );
      rows = rows.filter((n) => noteIds.has(n.id));
    }

    const sort = filter.sort ?? 'default';
    rows.sort((a, b) => {
      if (sort === 'default') {
        if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
        if (a.position !== b.position) return b.position - a.position;
        return b.updatedAt.localeCompare(a.updatedAt);
      }
      if (sort === 'created') return b.createdAt.localeCompare(a.createdAt);
      return b.updatedAt.localeCompare(a.updatedAt);
    });

    const offset = Math.max(0, filter.offset ?? 0);
    const limit = Math.min(Math.max(1, filter.limit ?? 200), 1000);
    return rows.slice(offset, offset + limit).map((r) => this.toDto(r));
  }

  update(userId: string, id: string, patch: UpdateNoteInput): Note | undefined {
    const row = this.findRow(userId, id);
    if (!row) return undefined;

    if (patch.title !== undefined) row.title = patch.title;
    if (patch.content !== undefined) row.content = patch.content;
    if (patch.color !== undefined) row.color = patch.color;
    if (patch.position !== undefined) row.position = patch.position;

    // Pinning an archived note unarchives it (Keep behaviour).
    if (patch.pinned !== undefined) {
      row.pinned = patch.pinned;
      if (patch.pinned && row.archived && patch.archived === undefined) {
        row.archived = false;
      }
    }
    if (patch.archived !== undefined) row.archived = patch.archived;

    if (patch.labels !== undefined) {
      this.setLabels(userId, id, patch.labels);
    }

    row.updatedAt = new Date().toISOString();
    this.store.schedulePersist();
    return this.toDto(row);
  }

  softDelete(userId: string, id: string): Note | undefined {
    const row = this.findRow(userId, id);
    if (!row) return undefined;
    row.trashed = true;
    row.updatedAt = new Date().toISOString();
    this.store.schedulePersist();
    return this.toDto(row);
  }

  restore(userId: string, id: string): Note | undefined {
    const row = this.findRow(userId, id);
    if (!row) return undefined;
    row.trashed = false;
    row.archived = false;
    row.updatedAt = new Date().toISOString();
    this.store.schedulePersist();
    return this.toDto(row);
  }

  /** Hard-delete a note and its label associations. */
  purge(userId: string, id: string): boolean {
    const idx = this.store.data.notes.findIndex((n) => n.id === id && n.userId === userId);
    if (idx === -1) return false;
    this.store.data.notes.splice(idx, 1);
    this.store.data.noteLabels = this.store.data.noteLabels.filter((nl) => nl.noteId !== id);
    this.store.schedulePersist();
    return true;
  }
}
