import { describe, it, expect, beforeEach } from 'vitest';
import request from 'supertest';
import { makeTestApp, loginAgent, type TestCtx } from './helpers/makeTestApp';

describe('Labels API', () => {
  let ctx: TestCtx;
  beforeEach(() => {
    ctx = makeTestApp();
  });

  it('CRUD: create, list, delete', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    const created = await agent.post('/api/labels').send({ name: 'work' });
    expect(created.status).toBe(201);
    expect(created.body.label.name).toBe('work');

    const list = await agent.get('/api/labels');
    expect(list.body.labels).toHaveLength(1);

    await agent.delete('/api/labels/' + created.body.label.id).expect(200);
    expect((await agent.get('/api/labels')).body.labels).toHaveLength(0);
  });

  it('auto-creates unknown labels when a note is created', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    await agent.post('/api/notes').send({ content: 'x', labels: ['new1', 'new2'] }).expect(201);
    const list = await agent.get('/api/labels');
    expect(list.body.labels.map((l: { name: string }) => l.name).sort()).toEqual(['new1', 'new2']);
  });

  it('labels are isolated per user', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const bob = await loginAgent(ctx.app, 'bob');
    await alice.post('/api/labels').send({ name: 'alice-only' });
    expect((await bob.get('/api/labels')).body.labels).toHaveLength(0);
  });

  it('requires authentication', async () => {
    await request(ctx.app).get('/api/labels').expect(401);
  });

  it('deleting an unknown label -> 404', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    await agent.delete('/api/labels/nope').expect(404);
  });
});
