import { describe, it, expect, beforeEach } from 'vitest';
import { makeTestRepos } from './helpers/makeTestApp';
import type { Repos } from '../src/app';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

describe('NoteRepo', () => {
  let repos: Repos;
  let userA: string;
  let userB: string;

  beforeEach(() => {
    ({ repos } = makeTestRepos());
    userA = repos.users.upsertFromOAuth({ provider: 'local', providerId: 'a' }).id;
    userB = repos.users.upsertFromOAuth({ provider: 'local', providerId: 'b' }).id;
  });

  it('create persists a note with uuid + timestamps', () => {
    const n = repos.notes.create(userA, { title: 'Hi', content: 'there' });
    expect(n.id).toMatch(UUID_RE);
    expect(n.title).toBe('Hi');
    expect(n.color).toBe('default');
    expect(n.pinned).toBe(false);
    expect(n.createdAt).toBeTruthy();
    expect(n.updatedAt).toBe(n.createdAt);
    expect(repos.notes.get(userA, n.id)).toBeDefined();
  });

  it('get returns own note, undefined for other user', () => {
    const n = repos.notes.create(userA, { title: 'secret' });
    expect(repos.notes.get(userA, n.id)).toBeDefined();
    expect(repos.notes.get(userB, n.id)).toBeUndefined();
  });

  it('list isolates per user', () => {
    repos.notes.create(userA, { title: 'a1' });
    repos.notes.create(userB, { title: 'b1' });
    expect(repos.notes.list(userA)).toHaveLength(1);
    expect(repos.notes.list(userB)).toHaveLength(1);
    expect(repos.notes.list(userA)[0].title).toBe('a1');
  });

  it('list filters archived / trashed / pinned', () => {
    const a = repos.notes.create(userA, { title: 'active' });
    const ar = repos.notes.create(userA, { title: 'arch' });
    repos.notes.update(userA, ar.id, { archived: true });
    const tr = repos.notes.create(userA, { title: 'trash' });
    repos.notes.softDelete(userA, tr.id);
    repos.notes.update(userA, a.id, { pinned: true });

    expect(repos.notes.list(userA).map((n) => n.title)).toEqual(['active']);
    expect(repos.notes.list(userA, { archived: true }).map((n) => n.title)).toEqual(['arch']);
    expect(repos.notes.list(userA, { trashed: true }).map((n) => n.title)).toEqual(['trash']);
    expect(repos.notes.list(userA, { pinned: true }).map((n) => n.title)).toEqual(['active']);
    expect(repos.notes.list(userA, { pinned: false })).toHaveLength(0);
  });

  it('search q matches title and content, case-insensitive', () => {
    repos.notes.create(userA, { title: 'Groceries', content: 'milk and EGGS' });
    repos.notes.create(userA, { title: 'Work', content: 'standup' });
    expect(repos.notes.list(userA, { q: 'grocer' })).toHaveLength(1);
    expect(repos.notes.list(userA, { q: 'eggs' })).toHaveLength(1);
    expect(repos.notes.list(userA, { q: 'STANDUP' })).toHaveLength(1);
    expect(repos.notes.list(userA, { q: 'nope' })).toHaveLength(0);
  });

  it('ordering: pinned first, then position desc, then updatedAt desc', () => {
    const n1 = repos.notes.create(userA, { title: 'first' });
    const n2 = repos.notes.create(userA, { title: 'second' });
    const n3 = repos.notes.create(userA, { title: 'third' });
    repos.notes.update(userA, n1.id, { pinned: true });
    const order = repos.notes.list(userA).map((n) => n.title);
    // n1 pinned -> first; remaining by position desc (n3 created last => highest position)
    expect(order[0]).toBe('first');
    expect(order.slice(1)).toEqual(['third', 'second']);
    expect(n2).toBeDefined();
    expect(n3).toBeDefined();
  });

  it('update mutates only provided fields and bumps updated_at', async () => {
    const n = repos.notes.create(userA, { title: 'orig', content: 'body' });
    await new Promise((r) => setTimeout(r, 5));
    const u = repos.notes.update(userA, n.id, { title: 'new' })!;
    expect(u.title).toBe('new');
    expect(u.content).toBe('body');
    expect(u.updatedAt >= n.updatedAt).toBe(true);
  });

  it('pinning an archived note unarchives it', () => {
    const n = repos.notes.create(userA, { title: 'x', archived: true });
    const u = repos.notes.update(userA, n.id, { pinned: true })!;
    expect(u.pinned).toBe(true);
    expect(u.archived).toBe(false);
  });

  it('softDelete / restore / purge transitions', () => {
    const n = repos.notes.create(userA, { title: 'x' });
    expect(repos.notes.softDelete(userA, n.id)!.trashed).toBe(true);
    expect(repos.notes.list(userA)).toHaveLength(0);
    expect(repos.notes.restore(userA, n.id)!.trashed).toBe(false);
    expect(repos.notes.list(userA)).toHaveLength(1);
    repos.notes.softDelete(userA, n.id);
    expect(repos.notes.purge(userA, n.id)).toBe(true);
    expect(repos.notes.get(userA, n.id)).toBeUndefined();
  });

  it('labels attach, filter, and auto-create', () => {
    const n = repos.notes.create(userA, { title: 'x', labels: ['work', 'urgent'] });
    expect(n.labels).toEqual(['urgent', 'work']);
    expect(repos.labels.listForUser(userA).map((l) => l.name)).toEqual(['urgent', 'work']);
    expect(repos.notes.list(userA, { label: 'work' })).toHaveLength(1);
    expect(repos.notes.list(userA, { label: 'missing' })).toHaveLength(0);
  });

  it('cross-user update/delete is a no-op (returns undefined)', () => {
    const n = repos.notes.create(userA, { title: 'x' });
    expect(repos.notes.update(userB, n.id, { title: 'hack' })).toBeUndefined();
    expect(repos.notes.softDelete(userB, n.id)).toBeUndefined();
    expect(repos.notes.purge(userB, n.id)).toBe(false);
  });
});
