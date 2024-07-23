import request from 'supertest';
import type { Express } from 'express';
import { makeTestApp, type AppBundle } from '../src/app.js';

export function setup(): AppBundle {
  return makeTestApp();
}

/** Register a user over HTTP and return their bearer token + id. */
export async function registerUser(
  app: Express,
  email = `user_${Math.random().toString(36).slice(2)}@example.com`,
  password = 'secret123',
): Promise<{ token: string; userId: string; email: string }> {
  const res = await request(app)
    .post('/api/auth/register')
    .send({ email, password });
  if (res.status !== 201) {
    throw new Error(`register failed: ${res.status} ${JSON.stringify(res.body)}`);
  }
  return { token: res.body.token, userId: res.body.user.id, email };
}

export function auth(token: string): string {
  return `Bearer ${token}`;
}

/** Direct-service core for unit tests (no HTTP), with a seeded user + page. */
export function makeCore(): {
  bundle: AppBundle;
  userId: string;
  pageId: string;
} {
  const bundle = makeTestApp();
  const { user } = bundle.services.auth.register('u@example.com', 'secret123');
  const page = bundle.services.pages.create(user.id, {});
  return { bundle, userId: user.id, pageId: page.id };
}
