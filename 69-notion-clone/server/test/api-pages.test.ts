import { describe, it, expect } from 'vitest';
import request from 'supertest';
import { setup, registerUser, auth } from './helpers.js';

describe('Pages API', () => {
  it('creates a page with default title and a seed paragraph block', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const res = await request(app)
      .post('/api/pages')
      .set('authorization', auth(token))
      .send({});
    expect(res.status).toBe(201);
    expect(res.body.title).toBe('Untitled');

    const detail = await request(app)
      .get(`/api/pages/${res.body.id}`)
      .set('authorization', auth(token));
    expect(detail.status).toBe(200);
    expect(detail.body.blocks).toHaveLength(1);
    expect(detail.body.blocks[0].type).toBe('paragraph');
  });

  it('renames a page and falls back to Untitled on blank title', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const page = (
      await request(app).post('/api/pages').set('authorization', auth(token)).send({})
    ).body;

    const renamed = await request(app)
      .patch(`/api/pages/${page.id}`)
      .set('authorization', auth(token))
      .send({ title: 'My Notes' });
    expect(renamed.body.title).toBe('My Notes');

    const blank = await request(app)
      .patch(`/api/pages/${page.id}`)
      .set('authorization', auth(token))
      .send({ title: '   ' });
    expect(blank.body.title).toBe('Untitled');
  });

  it('lists pages and supports nesting (R8)', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const parent = (
      await request(app).post('/api/pages').set('authorization', auth(token)).send({ title: 'Parent' })
    ).body;
    const child = (
      await request(app)
        .post('/api/pages')
        .set('authorization', auth(token))
        .send({ title: 'Child', parentPageId: parent.id })
    ).body;
    expect(child.parentPageId).toBe(parent.id);

    const list = await request(app).get('/api/pages').set('authorization', auth(token));
    expect(list.body).toHaveLength(2);
  });

  it('duplicates a page deep-copying blocks with a "Copy of" title (R7)', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const page = (
      await request(app).post('/api/pages').set('authorization', auth(token)).send({ title: 'Orig' })
    ).body;
    await request(app)
      .post(`/api/pages/${page.id}/blocks`)
      .set('authorization', auth(token))
      .send({ type: 'heading_1', text: 'Hello' });

    const dup = await request(app)
      .post(`/api/pages/${page.id}/duplicate`)
      .set('authorization', auth(token));
    expect(dup.status).toBe(201);
    expect(dup.body.title).toBe('Copy of Orig');
    expect(dup.body.id).not.toBe(page.id);

    const dupDetail = await request(app)
      .get(`/api/pages/${dup.body.id}`)
      .set('authorization', auth(token));
    const texts = dupDetail.body.blocks.map((b: { text: string }) => b.text);
    expect(texts).toContain('Hello');
  });

  it('reorders pages within the sidebar', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const p1 = (await request(app).post('/api/pages').set('authorization', auth(token)).send({ title: 'P1' })).body;
    const p2 = (await request(app).post('/api/pages').set('authorization', auth(token)).send({ title: 'P2' })).body;

    // move p1 to after p2
    const res = await request(app)
      .post(`/api/pages/${p1.id}/reorder`)
      .set('authorization', auth(token))
      .send({ afterPageId: p2.id });
    expect(res.status).toBe(200);
    expect(res.body.position).toBeGreaterThan(p2.position);
  });

  it('deletes a page and cascades to child pages', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const parent = (await request(app).post('/api/pages').set('authorization', auth(token)).send({})).body;
    await request(app)
      .post('/api/pages')
      .set('authorization', auth(token))
      .send({ parentPageId: parent.id });

    const del = await request(app)
      .delete(`/api/pages/${parent.id}`)
      .set('authorization', auth(token));
    expect(del.status).toBe(204);
    const list = await request(app).get('/api/pages').set('authorization', auth(token));
    expect(list.body).toHaveLength(0);
  });

  it("returns 404 for another user's page (no existence leak)", async () => {
    const { app } = setup();
    const a = await registerUser(app, 'a@x.com');
    const b = await registerUser(app, 'b@x.com');
    const page = (await request(app).post('/api/pages').set('authorization', auth(a.token)).send({})).body;
    const res = await request(app)
      .get(`/api/pages/${page.id}`)
      .set('authorization', auth(b.token));
    expect(res.status).toBe(404);
  });
});
