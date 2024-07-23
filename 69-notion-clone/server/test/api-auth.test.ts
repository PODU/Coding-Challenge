import { describe, it, expect } from 'vitest';
import request from 'supertest';
import { setup, auth } from './helpers.js';

describe('Auth API', () => {
  it('registers, logs in, and accesses a protected route', async () => {
    const { app } = setup();
    const reg = await request(app)
      .post('/api/auth/register')
      .send({ email: 'a@b.com', password: 'secret123' });
    expect(reg.status).toBe(201);
    expect(reg.body.token).toBeTruthy();
    expect(reg.body.user.email).toBe('a@b.com');

    const login = await request(app)
      .post('/api/auth/login')
      .send({ email: 'a@b.com', password: 'secret123' });
    expect(login.status).toBe(200);

    const me = await request(app)
      .get('/api/auth/me')
      .set('authorization', auth(login.body.token));
    expect(me.status).toBe(200);
    expect(me.body.user.email).toBe('a@b.com');
  });

  it('rejects wrong password with 401', async () => {
    const { app } = setup();
    await request(app)
      .post('/api/auth/register')
      .send({ email: 'a@b.com', password: 'secret123' });
    const res = await request(app)
      .post('/api/auth/login')
      .send({ email: 'a@b.com', password: 'wrong' });
    expect(res.status).toBe(401);
    expect(res.body.error.code).toBe('UNAUTHENTICATED');
  });

  it('rejects duplicate email with 409', async () => {
    const { app } = setup();
    await request(app)
      .post('/api/auth/register')
      .send({ email: 'a@b.com', password: 'secret123' });
    const res = await request(app)
      .post('/api/auth/register')
      .send({ email: 'a@b.com', password: 'secret123' });
    expect(res.status).toBe(409);
    expect(res.body.error.code).toBe('CONFLICT');
  });

  it('rejects missing token with 401 and invalid body with 400', async () => {
    const { app } = setup();
    expect((await request(app).get('/api/auth/me')).status).toBe(401);
    const bad = await request(app)
      .post('/api/auth/register')
      .send({ email: 'not-an-email', password: 'x' });
    expect(bad.status).toBe(400);
    expect(bad.body.error.code).toBe('VALIDATION_ERROR');
  });
});
