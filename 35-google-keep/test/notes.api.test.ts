import { describe, it, expect, beforeEach } from 'vitest';
import request from 'supertest';
import { makeTestApp, loginAgent, type TestCtx } from './helpers/makeTestApp';

describe('Notes API', () => {
  let ctx: TestCtx;

  beforeEach(() => {
    ctx = makeTestApp();
  });

  it('POST /api/notes creates a note owned by the session user (201)', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    const res = await agent.post('/api/notes').send({ title: 'Buy milk', content: 'soon' });
    expect(res.status).toBe(201);
    expect(res.body.note.id).toBeTruthy();
    expect(res.body.note.title).toBe('Buy milk');
    expect(res.body.note.position).toBeGreaterThan(0);
  });

  it('POST /api/notes with empty title+content -> 400 VALIDATION_ERROR', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    const res = await agent.post('/api/notes').send({ title: '', content: '' });
    expect(res.status).toBe(400);
    expect(res.body.error.code).toBe('VALIDATION_ERROR');
  });

  it('POST /api/notes rejects oversized content -> 400', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    const res = await agent.post('/api/notes').send({ content: 'x'.repeat(10001) });
    expect(res.status).toBe(400);
    expect(res.body.error.code).toBe('VALIDATION_ERROR');
  });

  it('POST /api/notes rejects unknown color -> 400', async () => {
    const agent = await loginAgent(ctx.app, 'alice');
    const res = await agent.post('/api/notes').send({ content: 'hi', color: 'chartreuse' });
    expect(res.status).toBe(400);
  });

  it('GET /api/notes/:id own->200, other user->404, bad uuid->404', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const bob = await loginAgent(ctx.app, 'bob');
    const { body } = await alice.post('/api/notes').send({ content: 'mine' });
    const id = body.note.id;

    await alice.get('/api/notes/' + id).expect(200);
    await bob.get('/api/notes/' + id).expect(404);
    await alice.get('/api/notes/not-a-uuid').expect(404);
  });

  it('GET /api/notes returns only the callers notes and respects filters', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const bob = await loginAgent(ctx.app, 'bob');
    await alice.post('/api/notes').send({ title: 'Apple pie', labels: ['food'] });
    await alice.post('/api/notes').send({ title: 'Meeting' });
    await bob.post('/api/notes').send({ title: 'Bob note' });

    const all = await alice.get('/api/notes');
    expect(all.body.notes).toHaveLength(2);

    const search = await alice.get('/api/notes?q=apple');
    expect(search.body.notes).toHaveLength(1);
    expect(search.body.notes[0].title).toBe('Apple pie');

    const byLabel = await alice.get('/api/notes?label=food');
    expect(byLabel.body.notes).toHaveLength(1);
  });

  it('PATCH /api/notes/:id partial update; pin unarchives', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const { body } = await alice.post('/api/notes').send({ title: 't', archived: true });
    const id = body.note.id;
    const res = await alice.patch('/api/notes/' + id).send({ pinned: true });
    expect(res.status).toBe(200);
    expect(res.body.note.pinned).toBe(true);
    expect(res.body.note.archived).toBe(false);
  });

  it('PATCH with no fields -> 400', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const { body } = await alice.post('/api/notes').send({ content: 'x' });
    await alice.patch('/api/notes/' + body.note.id).send({}).expect(400);
  });

  it('DELETE soft-deletes, restore brings back, ?purge=1 hard-deletes', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const { body } = await alice.post('/api/notes').send({ content: 'x' });
    const id = body.note.id;

    const del = await alice.delete('/api/notes/' + id);
    expect(del.body.note.trashed).toBe(true);
    expect((await alice.get('/api/notes')).body.notes).toHaveLength(0);
    expect((await alice.get('/api/notes?trashed=1')).body.notes).toHaveLength(1);

    await alice.post('/api/notes/' + id + '/restore').expect(200);
    expect((await alice.get('/api/notes')).body.notes).toHaveLength(1);

    await alice.delete('/api/notes/' + id);
    const purge = await alice.delete('/api/notes/' + id + '?purge=1');
    expect(purge.body.purged).toBe(true);
    await alice.get('/api/notes/' + id).expect(404);
  });

  it('cross-user mutation attempts -> 404', async () => {
    const alice = await loginAgent(ctx.app, 'alice');
    const bob = await loginAgent(ctx.app, 'bob');
    const { body } = await alice.post('/api/notes').send({ content: 'private' });
    const id = body.note.id;
    await bob.patch('/api/notes/' + id).send({ title: 'hacked' }).expect(404);
    await bob.delete('/api/notes/' + id).expect(404);
  });

  it('all /api/notes* without a session -> 401', async () => {
    await request(ctx.app).get('/api/notes').expect(401);
    await request(ctx.app).post('/api/notes').send({ content: 'x' }).expect(401);
    await request(ctx.app).get('/api/notes/whatever').expect(401);
  });
});
