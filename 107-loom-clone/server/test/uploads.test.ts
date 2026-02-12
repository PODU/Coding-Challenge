import { afterEach, describe, it, expect } from 'vitest';
import request from 'supertest';
import { makeContext, uploadChunks, type TestContext } from './helpers';

let ctx: TestContext;

afterEach(async () => {
  await ctx?.cleanup();
});

describe('upload session lifecycle', () => {
  it('create → append chunks → complete assembles the blob and creates a Recording with shareId', async () => {
    ctx = await makeContext();
    const chunks = [Buffer.from('hello '), Buffer.from('brave '), Buffer.from('world')];
    const { recording } = await uploadChunks(ctx.app, chunks, { title: 'My clip', durationMs: 4200 });

    expect(recording.id).toBeTypeOf('string');
    expect(recording.shareId).toMatch(/^[A-Za-z0-9_-]{22}$/);
    expect(recording.title).toBe('My clip');
    expect(recording.durationMs).toBe(4200);
    expect(recording.sizeBytes).toBe(17); // 'hello brave world'
    expect(recording.shareUrl).toBe(`/v/${recording.shareId}`);

    // Blob bytes equal the concatenation of the chunks.
    const key = recording.filename as string;
    expect(ctx.storage.read(key)?.toString()).toBe('hello brave world');
    // Service streamed into storage rather than buffering a whole file.
    expect(ctx.storage.streamingPutUsed).toBe(true);
  });

  it('defaults the title when none is provided', async () => {
    ctx = await makeContext();
    const { recording } = await uploadChunks(ctx.app, [Buffer.from('x')]);
    expect(recording.title).toMatch(/^Recording \d{4}-/);
  });
});

describe('resume', () => {
  it('reports nextExpectedIndex and finishes after a reconnect', async () => {
    ctx = await makeContext();
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'c.webm', mimeType: 'video/webm' })
      .expect(201);
    const uploadId = body.uploadId as string;

    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 0 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('AA')).expect(204);
    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 1 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('BB')).expect(204);

    const status = await agent.get(`/api/uploads/${uploadId}`).expect(200);
    expect(status.body.receivedChunks).toEqual([0, 1]);
    expect(status.body.nextExpectedIndex).toBe(2);

    // Reconnect and finish.
    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 2 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('CC')).expect(204);
    const complete = await agent.post(`/api/uploads/${uploadId}/complete`).send({}).expect(201);
    expect(ctx.storage.read(complete.body.filename as string)?.toString()).toBe('AABBCC');
  });
});

describe('chunk ordering', () => {
  it('rejects a gap with 409 and ignores a duplicate index', async () => {
    ctx = await makeContext();
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'c.webm', mimeType: 'video/webm' })
      .expect(201);
    const uploadId = body.uploadId as string;

    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 0 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('first')).expect(204);

    // Gap: expected 1, got 2.
    const gap = await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 2 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('skip')).expect(409);
    expect(gap.body.error.code).toBe('CHUNK_GAP');

    // Duplicate of index 0 → ignored (204), no corruption.
    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 0 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('DUPLICATE')).expect(204);

    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 1 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('second')).expect(204);

    const complete = await agent.post(`/api/uploads/${uploadId}/complete`).send({}).expect(201);
    expect(ctx.storage.read(complete.body.filename as string)?.toString()).toBe('firstsecond');
  });
});

describe('size limit and empty upload', () => {
  it('rejects an over-limit upload with 413', async () => {
    ctx = await makeContext({ maxBytes: 8 });
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'big.webm', mimeType: 'video/webm' })
      .expect(201);
    const uploadId = body.uploadId as string;

    const res = await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 0 })
      .set('Content-Type', 'application/octet-stream')
      .send(Buffer.alloc(16, 1))
      .expect(413);
    expect(res.body.error.code).toBe('PAYLOAD_TOO_LARGE');
  });

  it('rejects completing an empty upload with 400', async () => {
    ctx = await makeContext();
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'empty.webm', mimeType: 'video/webm' })
      .expect(201);
    const res = await agent.post(`/api/uploads/${body.uploadId}/complete`).send({}).expect(400);
    expect(res.body.error.code).toBe('EMPTY_UPLOAD');
  });
});

describe('idempotent complete', () => {
  it('returns the same Recording on a second complete', async () => {
    ctx = await makeContext();
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'c.webm', mimeType: 'video/webm' })
      .expect(201);
    const uploadId = body.uploadId as string;
    await agent.put(`/api/uploads/${uploadId}/chunk`).query({ index: 0 })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('data')).expect(204);

    const first = await agent.post(`/api/uploads/${uploadId}/complete`).send({}).expect(201);
    const second = await agent.post(`/api/uploads/${uploadId}/complete`).send({}).expect(201);
    expect(second.body.id).toBe(first.body.id);
    expect(second.body.shareId).toBe(first.body.shareId);

    const list = await agent.get('/api/recordings').expect(200);
    expect(list.body.items).toHaveLength(1);
  });
});

describe('validation', () => {
  it('rejects a malformed create body with 400', async () => {
    ctx = await makeContext();
    const res = await request(ctx.app).post('/api/uploads').send({ mimeType: 'video/webm' }).expect(400);
    expect(res.body.error.code).toBe('VALIDATION');
  });

  it('rejects a non-numeric chunk index with 400', async () => {
    ctx = await makeContext();
    const agent = request(ctx.app);
    const { body } = await agent
      .post('/api/uploads')
      .send({ filename: 'c.webm', mimeType: 'video/webm' })
      .expect(201);
    await agent.put(`/api/uploads/${body.uploadId}/chunk`).query({ index: 'abc' })
      .set('Content-Type', 'application/octet-stream').send(Buffer.from('x')).expect(400);
  });

  it('returns 404 for an unknown upload session', async () => {
    ctx = await makeContext();
    await request(ctx.app).get('/api/uploads/does-not-exist').expect(404);
  });
});
