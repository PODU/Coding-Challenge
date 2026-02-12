import { afterEach, describe, it, expect } from 'vitest';
import request from 'supertest';
import { makeContext, uploadChunks, type TestContext } from './helpers';

let ctx: TestContext;

afterEach(async () => {
  await ctx?.cleanup();
});

describe('list & get', () => {
  it('returns recordings newest-first', async () => {
    ctx = await makeContext();
    const a = await uploadChunks(ctx.app, [Buffer.from('a')], { title: 'first' });
    // Ensure a distinct, later createdAt for ordering.
    await new Promise((r) => setTimeout(r, 5));
    const b = await uploadChunks(ctx.app, [Buffer.from('b')], { title: 'second' });

    const res = await request(ctx.app).get('/api/recordings').expect(200);
    const ids = (res.body.items as Array<{ id: string }>).map((r) => r.id);
    expect(ids[0]).toBe(b.recording.id);
    expect(ids[1]).toBe(a.recording.id);
  });

  it('paginates with a cursor', async () => {
    ctx = await makeContext();
    for (let i = 0; i < 5; i += 1) {
      await uploadChunks(ctx.app, [Buffer.from(`c${i}`)], { title: `clip ${i}` });
      await new Promise((r) => setTimeout(r, 2));
    }
    const page1 = await request(ctx.app).get('/api/recordings').query({ limit: 2 }).expect(200);
    expect(page1.body.items).toHaveLength(2);
    expect(page1.body.nextCursor).toBeTypeOf('string');

    const page2 = await request(ctx.app)
      .get('/api/recordings')
      .query({ limit: 2, cursor: page1.body.nextCursor })
      .expect(200);
    expect(page2.body.items).toHaveLength(2);

    const ids1 = (page1.body.items as Array<{ id: string }>).map((r) => r.id);
    const ids2 = (page2.body.items as Array<{ id: string }>).map((r) => r.id);
    expect(ids1.some((id) => ids2.includes(id))).toBe(false);
  });

  it('gets one recording and 404s on unknown id', async () => {
    ctx = await makeContext();
    const { recording } = await uploadChunks(ctx.app, [Buffer.from('x')]);
    const got = await request(ctx.app).get(`/api/recordings/${recording.id}`).expect(200);
    expect(got.body.id).toBe(recording.id);
    await request(ctx.app).get('/api/recordings/nope').expect(404);
  });
});

describe('delete', () => {
  it('removes blob + metadata, then get/stream 404, and is idempotent', async () => {
    ctx = await makeContext();
    const { recording } = await uploadChunks(ctx.app, [Buffer.from('payload')]);
    const id = recording.id as string;
    const key = recording.filename as string;
    expect(ctx.storage.read(key)).toBeDefined();

    await request(ctx.app).delete(`/api/recordings/${id}`).expect(204);
    expect(ctx.storage.read(key)).toBeUndefined();

    await request(ctx.app).get(`/api/recordings/${id}`).expect(404);
    await request(ctx.app).get(`/api/recordings/${id}/stream`).expect(404);

    // Idempotent second delete.
    await request(ctx.app).delete(`/api/recordings/${id}`).expect(204);
  });
});

describe('range streaming', () => {
  async function seeded(): Promise<{ id: string; body: Buffer }> {
    const body = Buffer.from('0123456789ABCDEF'.repeat(16)); // 256 bytes
    const { recording } = await uploadChunks(ctx.app, [body], { mimeType: 'video/webm' });
    return { id: recording.id as string, body };
  }

  it('serves a 206 partial slice with correct headers', async () => {
    ctx = await makeContext();
    const { id, body } = await seeded();
    const res = await request(ctx.app)
      .get(`/api/recordings/${id}/stream`)
      .set('Range', 'bytes=0-99')
      .expect(206);

    expect(res.headers['content-range']).toBe(`bytes 0-99/${body.length}`);
    expect(res.headers['content-length']).toBe('100');
    expect(res.headers['accept-ranges']).toBe('bytes');
    expect(res.headers['content-type']).toContain('video/webm');
    expect(Buffer.from(res.body).length).toBe(100);
    expect(Buffer.from(res.body).equals(body.subarray(0, 100))).toBe(true);
  });

  it('serves a 206 mid-file slice', async () => {
    ctx = await makeContext();
    const { id, body } = await seeded();
    const res = await request(ctx.app)
      .get(`/api/recordings/${id}/stream`)
      .set('Range', 'bytes=100-149')
      .expect(206);
    expect(res.headers['content-range']).toBe(`bytes 100-149/${body.length}`);
    expect(Buffer.from(res.body).equals(body.subarray(100, 150))).toBe(true);
  });

  it('serves the full body with 200 when no Range header', async () => {
    ctx = await makeContext();
    const { id, body } = await seeded();
    const res = await request(ctx.app).get(`/api/recordings/${id}/stream`).expect(200);
    expect(res.headers['content-length']).toBe(String(body.length));
    expect(res.headers['accept-ranges']).toBe('bytes');
    expect(Buffer.from(res.body).equals(body)).toBe(true);
  });

  it('returns 416 for an unsatisfiable range', async () => {
    ctx = await makeContext();
    const { id, body } = await seeded();
    const res = await request(ctx.app)
      .get(`/api/recordings/${id}/stream`)
      .set('Range', 'bytes=99999-100000')
      .expect(416);
    expect(res.headers['content-range']).toBe(`bytes */${body.length}`);
  });
});
