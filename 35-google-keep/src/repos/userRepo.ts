import { randomUUID } from 'node:crypto';
import type { Store } from '../store/store';
import type { User } from '../types';

export interface UpsertOAuthInput {
  provider: 'google' | 'local';
  providerId: string;
  email?: string | null;
  displayName?: string | null;
}

export class UserRepo {
  constructor(private readonly store: Store) {}

  getById(id: string): User | undefined {
    return this.store.data.users.find((u) => u.id === id);
  }

  findByProvider(provider: string, providerId: string): User | undefined {
    return this.store.data.users.find(
      (u) => u.provider === provider && u.providerId === providerId,
    );
  }

  /**
   * Idempotent on (provider, providerId): returns the existing user if present,
   * otherwise creates one. Mirrors the OAuth login flow.
   */
  upsertFromOAuth(input: UpsertOAuthInput): User {
    const existing = this.findByProvider(input.provider, input.providerId);
    if (existing) {
      let changed = false;
      if (input.email !== undefined && input.email !== existing.email) {
        existing.email = input.email;
        changed = true;
      }
      if (input.displayName !== undefined && input.displayName !== existing.displayName) {
        existing.displayName = input.displayName;
        changed = true;
      }
      if (changed) this.store.schedulePersist();
      return existing;
    }
    const user: User = {
      id: randomUUID(),
      provider: input.provider,
      providerId: input.providerId,
      email: input.email ?? null,
      displayName: input.displayName ?? null,
      createdAt: new Date().toISOString(),
    };
    this.store.data.users.push(user);
    this.store.schedulePersist();
    return user;
  }
}
