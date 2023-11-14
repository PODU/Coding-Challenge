import { randomUUID } from 'node:crypto';
import type { Store } from '../store/store';
import type { Label } from '../types';

export class LabelRepo {
  constructor(private readonly store: Store) {}

  listForUser(userId: string): Label[] {
    return this.store.data.labels
      .filter((l) => l.userId === userId)
      .sort((a, b) => a.name.localeCompare(b.name));
  }

  getById(userId: string, id: string): Label | undefined {
    return this.store.data.labels.find((l) => l.id === id && l.userId === userId);
  }

  findByName(userId: string, name: string): Label | undefined {
    return this.store.data.labels.find((l) => l.userId === userId && l.name === name);
  }

  /** Idempotent get-or-create for a label name, scoped to the user. */
  getOrCreate(userId: string, name: string): Label {
    const trimmed = name.trim();
    const existing = this.findByName(userId, trimmed);
    if (existing) return existing;
    const label: Label = { id: randomUUID(), userId, name: trimmed };
    this.store.data.labels.push(label);
    this.store.schedulePersist();
    return label;
  }

  create(userId: string, name: string): Label {
    return this.getOrCreate(userId, name);
  }

  /** Delete a label and cascade-remove its note associations. */
  delete(userId: string, id: string): boolean {
    const idx = this.store.data.labels.findIndex((l) => l.id === id && l.userId === userId);
    if (idx === -1) return false;
    this.store.data.labels.splice(idx, 1);
    this.store.data.noteLabels = this.store.data.noteLabels.filter((nl) => nl.labelId !== id);
    this.store.schedulePersist();
    return true;
  }
}
