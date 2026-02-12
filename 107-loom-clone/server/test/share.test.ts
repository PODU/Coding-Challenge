import { afterEach, describe, it, expect } from 'vitest';
import request from 'supertest';
import { makeContext, uploadChunks, type TestContext } from './helpers';

let ctx: TestContext;

afterEach(async () => {
  await ctx?.cleanup();
});

describe('share-link resolution', () => {
  it('resolves a share link to the public subset and never leaks the internal id or blob key', async () => {
    ctx = await makeContext();
    const { recording } = await uploadChunks(ctx.app, [Buffer.from('shared bytes')], {
      title: 'Public clip',
    });
    const shareId = recording.shareId as string;

    const res = await request(ctx.app).get(`/api/share/${shareId}`).expect(200);
    expect(res.body.shareId).toBe(shareId);
    expect(res.body.title).toBe('Public clip');
    expect(res.body.mimeType).toBe('video/webm');
    // Internal fields must not be present.
    expect(res.body.id).toBeUndefined();
    expect(res.body.filename).toBeUndefined();
  });

  it('returns 404 for an unknown shareId', async () => {
    ctx = await makeContext();
    const res = await request(ctx.app).get('/api/share/UNKNOWNSHAREID1234567x').expect(404);
    expect(res.body.error.code).toBe('NOT_FOUND');
  });

  it('streams the shared blob with Range support (206)', async () => {
    ctx = await makeContext();
    const body = Buffer.from('SHAREDDATA'.repeat(20)); // 200 bytes
    const { recording } = await uploadChunks(ctx.app, [body]);
    const shareId = recording.shareId as string;

    const res = await request(ctx.app)
      .get(`/api/share/${shareId}/stream`)
      .set('Range', 'bytes=0-9')
      .expect(206);
    expect(res.headers['content-range']).toBe(`bytes 0-9/${body.length}`);
    expect(Buffer.from(res.body).equals(body.subarray(0, 10))).toBe(true);
  });

  it('404s on the share stream after the recording is deleted', async () => {
    ctx = await makeContext();
    const { recording } = await uploadChunks(ctx.app, [Buffer.from('temp')]);
    await request(ctx.app).delete(`/api/recordings/${recording.id}`).expect(204);
    await request(ctx.app).get(`/api/share/${recording.shareId}`).expect(404);
    await request(ctx.app).get(`/api/share/${recording.shareId}/stream`).expect(404);
  });
});
