import { describe, it, expect } from 'vitest';
import request from 'supertest';
import type { Express } from 'express';
import { setup, registerUser, auth } from './helpers.js';

async function freshPage(app: Express, token: string): Promise<string> {
  const res = await request(app).post('/api/pages').set('authorization', auth(token)).send({});
  return res.body.id;
}

describe('Blocks API', () => {
  it('full CRUD: create, edit, change type, move, delete subtree', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const pageId = await freshPage(app, token);

    const create = await request(app)
      .post(`/api/pages/${pageId}/blocks`)
      .set('authorization', auth(token))
      .send({ type: 'paragraph', text: 'first' });
    expect(create.status).toBe(201);
    const blockId = create.body.id;

    const patch = await request(app)
      .patch(`/api/blocks/${blockId}`)
      .set('authorization', auth(token))
      .send({ text: 'edited' });
    expect(patch.body.text).toBe('edited');

    const changed = await request(app)
      .post(`/api/blocks/${blockId}/type`)
      .set('authorization', auth(token))
      .send({ type: 'heading_2' });
    expect(changed.body.type).toBe('heading_2');

    const child = await request(app)
      .post(`/api/pages/${pageId}/blocks`)
      .set('authorization', auth(token))
      .send({ type: 'paragraph', text: 'child', parentId: blockId });
    expect(child.status).toBe(201);

    const del = await request(app)
      .delete(`/api/blocks/${blockId}`)
      .set('authorization', auth(token));
    expect(del.status).toBe(204);

    const detail = await request(app)
      .get(`/api/pages/${pageId}`)
      .set('authorization', auth(token));
    const ids = detail.body.blocks.map((b: { id: string }) => b.id);
    expect(ids).not.toContain(blockId);
    expect(ids).not.toContain(child.body.id);
  });

  it('moves (indents) a block under another', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const pageId = await freshPage(app, token);
    const a = (await request(app).post(`/api/pages/${pageId}/blocks`).set('authorization', auth(token)).send({ type: 'bulleted_list_item', text: 'A' })).body;
    const b = (await request(app).post(`/api/pages/${pageId}/blocks`).set('authorization', auth(token)).send({ type: 'bulleted_list_item', text: 'B' })).body;

    const move = await request(app)
      .post(`/api/blocks/${b.id}/move`)
      .set('authorization', auth(token))
      .send({ parentId: a.id });
    expect(move.status).toBe(200);
    expect(move.body.parentId).toBe(a.id);
  });

  it('rejects an unknown block type with VALIDATION_ERROR', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const pageId = await freshPage(app, token);
    const res = await request(app)
      .post(`/api/pages/${pageId}/blocks`)
      .set('authorization', auth(token))
      .send({ type: 'banana', text: 'x' });
    expect(res.status).toBe(400);
    expect(res.body.error.code).toBe('VALIDATION_ERROR');
  });

  it('batch endpoint applies ops and returns the tree', async () => {
    const { app } = setup();
    const { token } = await registerUser(app);
    const pageId = await freshPage(app, token);
    const res = await request(app)
      .post(`/api/pages/${pageId}/blocks/batch`)
      .set('authorization', auth(token))
      .send({
        ops: [
          { op: 'create', type: 'paragraph', text: 'one' },
          { op: 'create', type: 'paragraph', text: 'two' },
        ],
      });
    expect(res.status).toBe(200);
    const texts = res.body.blocks.map((b: { text: string }) => b.text);
    expect(texts).toContain('one');
    expect(texts).toContain('two');
  });

  it("forbids editing another user's block (404)", async () => {
    const { app } = setup();
    const a = await registerUser(app, 'a@x.com');
    const b = await registerUser(app, 'b@x.com');
    const pageId = await freshPage(app, a.token);
    const block = (await request(app).post(`/api/pages/${pageId}/blocks`).set('authorization', auth(a.token)).send({ type: 'paragraph', text: 'mine' })).body;
    const res = await request(app)
      .patch(`/api/blocks/${block.id}`)
      .set('authorization', auth(b.token))
      .send({ text: 'hacked' });
    expect(res.status).toBe(404);
  });
});
