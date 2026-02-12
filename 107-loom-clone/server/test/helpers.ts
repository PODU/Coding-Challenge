import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import type { Express } from 'express';
import request from 'supertest';
import { createApp } from '../src/app';
import { MockStorage } from '../src/storage/MockStorage';
import { InMemoryRecordingsRepo } from '../src/repo/Recordings';
import { UploadService } from '../src/services/UploadService';
import { RecordingService } from '../src/services/RecordingService';
import { ShareService } from '../src/services/ShareService';

export interface TestContext {
  app: Express;
  storage: MockStorage;
  repo: InMemoryRecordingsRepo;
  uploadService: UploadService;
  uploadsDir: string;
  cleanup: () => Promise<void>;
}

export async function makeContext(opts?: { maxBytes?: number }): Promise<TestContext> {
  const baseDir = await mkdtemp(path.join(tmpdir(), 'loom-test-'));
  const uploadsDir = path.join(baseDir, 'uploads');

  const storage = new MockStorage();
  const repo = new InMemoryRecordingsRepo();
  const uploadService = new UploadService({
    uploadsDir,
    storage,
    repo,
    ...(opts?.maxBytes !== undefined ? { maxBytes: opts.maxBytes } : {}),
  });
  const recordingService = new RecordingService(repo, storage);
  const shareService = new ShareService(repo, storage);

  const app = createApp({ uploadService, recordingService, shareService, storage });

  return {
    app,
    storage,
    repo,
    uploadService,
    uploadsDir,
    cleanup: () => rm(baseDir, { recursive: true, force: true }),
  };
}

/** Drive a full create → append chunks → complete flow; returns the Recording JSON. */
export async function uploadChunks(
  app: Express,
  chunks: Buffer[],
  meta?: { filename?: string; mimeType?: string; title?: string; durationMs?: number },
): Promise<{ uploadId: string; recording: Record<string, unknown> }> {
  const agent = request(app);
  const create = await agent
    .post('/api/uploads')
    .send({
      filename: meta?.filename ?? 'clip.webm',
      mimeType: meta?.mimeType ?? 'video/webm',
    })
    .expect(201);
  const uploadId = create.body.uploadId as string;

  for (let i = 0; i < chunks.length; i += 1) {
    await agent
      .put(`/api/uploads/${uploadId}/chunk`)
      .query({ index: i })
      .set('Content-Type', 'application/octet-stream')
      .send(chunks[i] as Buffer)
      .expect(204);
  }

  const complete = await agent
    .post(`/api/uploads/${uploadId}/complete`)
    .send({
      ...(meta?.title !== undefined ? { title: meta.title } : {}),
      ...(meta?.durationMs !== undefined ? { durationMs: meta.durationMs } : {}),
    })
    .expect(201);

  return { uploadId, recording: complete.body as Record<string, unknown> };
}
