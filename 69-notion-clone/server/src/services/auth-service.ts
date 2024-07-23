import type { AuthResponse, User } from '../../../shared/types.js';
import type { Config } from '../config.js';
import { AppError } from '../errors.js';
import type { Store, UserRow } from '../store.js';
import {
  hashPassword,
  newId,
  signToken,
  verifyPassword,
  verifyToken,
} from '../util/crypto.js';

function toPublicUser(row: UserRow): User {
  return { id: row.id, email: row.email, createdAt: row.createdAt };
}

export class AuthService {
  constructor(
    private readonly store: Store,
    private readonly config: Config,
  ) {}

  register(email: string, password: string): AuthResponse {
    const normalized = email.trim().toLowerCase();
    if (this.store.getUserByEmail(normalized)) {
      throw AppError.conflict('Email already registered');
    }
    const row: UserRow = {
      id: newId(),
      email: normalized,
      passwordHash: hashPassword(password),
      createdAt: new Date().toISOString(),
    };
    this.store.insertUser(row);
    return this.authResponse(row);
  }

  login(email: string, password: string): AuthResponse {
    const row = this.store.getUserByEmail(email.trim().toLowerCase());
    if (!row || !verifyPassword(password, row.passwordHash)) {
      throw AppError.unauthenticated('Invalid email or password');
    }
    return this.authResponse(row);
  }

  /** Resolve a bearer token to a user id, or throw 401. */
  authenticate(token: string | undefined): string {
    if (!token) throw AppError.unauthenticated();
    const payload = verifyToken(token, this.config.jwtSecret);
    if (!payload) throw AppError.unauthenticated('Invalid or expired token');
    if (!this.store.getUserById(payload.sub)) {
      throw AppError.unauthenticated('Invalid or expired token');
    }
    return payload.sub;
  }

  getUser(userId: string): User {
    const row = this.store.getUserById(userId);
    if (!row) throw AppError.notFound('User not found');
    return toPublicUser(row);
  }

  private authResponse(row: UserRow): AuthResponse {
    const token = signToken(
      row.id,
      this.config.jwtSecret,
      this.config.jwtExpirySeconds,
    );
    return { user: toPublicUser(row), token };
  }
}
