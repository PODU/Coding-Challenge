import { describe, it, expect, beforeEach } from 'vitest';
import { makeTestRepos } from './helpers/makeTestApp';
import type { Repos } from '../src/app';

describe('LabelRepo', () => {
  let repos: Repos;
  let userA: string;

  beforeEach(() => {
    ({ repos } = makeTestRepos());
    userA = repos.users.upsertFromOAuth({ provider: 'local', providerId: 'a' }).id;
  });

  it('getOrCreate is idempotent per (user, name)', () => {
    const a = repos.labels.getOrCreate(userA, 'work');
    const b = repos.labels.getOrCreate(userA, 'work');
    expect(a.id).toBe(b.id);
    expect(repos.labels.listForUser(userA)).toHaveLength(1);
  });

  it('delete cascades to note_labels', () => {
    const n = repos.notes.create(userA, { title: 'x', labels: ['work'] });
    const label = repos.labels.findByName(userA, 'work')!;
    expect(repos.notes.get(userA, n.id)!.labels).toEqual(['work']);
    expect(repos.labels.delete(userA, label.id)).toBe(true);
    expect(repos.notes.get(userA, n.id)!.labels).toEqual([]);
    expect(repos.labels.listForUser(userA)).toHaveLength(0);
  });

  it('delete of unknown label returns false', () => {
    expect(repos.labels.delete(userA, 'nope')).toBe(false);
  });
});

describe('UserRepo', () => {
  it('upsertFromOAuth is idempotent on (provider, providerId)', () => {
    const { repos } = makeTestRepos();
    const u1 = repos.users.upsertFromOAuth({ provider: 'google', providerId: 'sub-1', email: 'a@x.com' });
    const u2 = repos.users.upsertFromOAuth({ provider: 'google', providerId: 'sub-1', email: 'a@x.com' });
    expect(u1.id).toBe(u2.id);
    const u3 = repos.users.upsertFromOAuth({ provider: 'google', providerId: 'sub-2' });
    expect(u3.id).not.toBe(u1.id);
  });
});
