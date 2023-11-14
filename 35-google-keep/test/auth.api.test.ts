import { describe, it, expect } from 'vitest';
import request from 'supertest';
import { makeTestApp } from './helpers/makeTestApp';
import type { GoogleAuthClient } from '../src/auth/google';

describe('Auth API', () => {
  it('local login issues a session cookie; /me then 200; logout clears it', async () => {
    const { app } = makeTestApp();
    const agent = request.agent(app);

    await agent.get('/api/auth/me').expect(401);

    const login = await agent.post('/api/auth/local').send({ username: 'alice' });
    expect(login.status).toBe(200);
    expect(login.headers['set-cookie'].join(';')).toContain('keep_session=');

    const me = await agent.get('/api/auth/me');
    expect(me.status).toBe(200);
    expect(me.body.user.displayName).toBe('alice');
    expect(me.body.user.provider).toBe('local');

    await agent.post('/api/auth/logout').expect(200);
    await agent.get('/api/auth/me').expect(401);
  });

  it('local login rejected when AUTH_LOCAL disabled -> 404', async () => {
    const { app } = makeTestApp({ env: { AUTH_LOCAL: '0' } });
    await request(app).post('/api/auth/local').send({ username: 'x' }).expect(404);
  });

  it('local login requires a username', async () => {
    const { app } = makeTestApp();
    await request(app).post('/api/auth/local').send({}).expect(400);
  });

  it('a tampered session cookie is rejected', async () => {
    const { app } = makeTestApp();
    await request(app)
      .get('/api/auth/me')
      .set('Cookie', 'keep_session=someuser.badsignature')
      .expect(401);
  });

  it('Google OAuth callback with a faked client logs the user in (no network)', async () => {
    const google: GoogleAuthClient = {
      getAuthUrl: (state, redirectUri) =>
        `https://accounts.example/auth?state=${state}&redirect_uri=${encodeURIComponent(redirectUri)}`,
      exchangeCode: async () => ({
        providerId: 'google-sub-123',
        email: 'g@example.com',
        displayName: 'Google User',
      }),
    };
    const { app, repos } = makeTestApp({ google });
    const agent = request.agent(app);

    const start = await agent.get('/api/auth/google');
    expect(start.status).toBe(302);
    const state = new URL(start.headers.location).searchParams.get('state');
    expect(state).toBeTruthy();

    const cb = await agent.get(`/api/auth/google/callback?code=auth-code&state=${state}`);
    expect(cb.status).toBe(302);
    expect(cb.headers.location).toBe('/');

    const me = await agent.get('/api/auth/me');
    expect(me.status).toBe(200);
    expect(me.body.user.email).toBe('g@example.com');
    expect(repos.users.findByProvider('google', 'google-sub-123')).toBeDefined();
  });

  it('Google callback rejects a mismatched state -> 400', async () => {
    const google: GoogleAuthClient = {
      getAuthUrl: (state) => `https://accounts.example/auth?state=${state}`,
      exchangeCode: async () => ({ providerId: 'x', email: null, displayName: null }),
    };
    const { app } = makeTestApp({ google });
    const agent = request.agent(app);
    await agent.get('/api/auth/google').expect(302);
    await agent.get('/api/auth/google/callback?code=c&state=wrong-state').expect(400);
  });

  it('Google endpoints 404 when not configured', async () => {
    const { app } = makeTestApp();
    await request(app).get('/api/auth/google').expect(404);
  });
});
