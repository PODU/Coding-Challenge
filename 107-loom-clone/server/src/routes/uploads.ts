import { Router } from 'express';
import { z } from 'zod';
import { ApiError, asyncHandler } from '../errors';
import type { UploadService } from '../services/UploadService';

const createSchema = z.object({
  filename: z.string().min(1).max(512),
  mimeType: z.string().min(1).max(255),
  totalBytes: z.number().int().nonnegative().optional(),
});

const completeSchema = z.object({
  title: z.string().max(255).optional(),
  durationMs: z.number().int().nonnegative().optional(),
});

const indexSchema = z.coerce.number().int().nonnegative();

function parse<T>(schema: z.ZodType<T>, value: unknown): T {
  const result = schema.safeParse(value);
  if (!result.success) {
    throw ApiError.badRequest('VALIDATION', result.error.issues[0]?.message ?? 'Invalid request');
  }
  return result.data;
}

export function buildUploadsRouter(uploadService: UploadService): Router {
  const router = Router();

  router.post(
    '/',
    asyncHandler(async (req, res) => {
      const body = parse(createSchema, req.body);
      const { uploadId } = await uploadService.createSession(body);
      res.status(201).json({ uploadId });
    }),
  );

  // The chunk body is piped straight to disk, never parsed.
  router.put(
    '/:uploadId/chunk',
    asyncHandler(async (req, res) => {
      const index = parse(indexSchema, req.query.index);
      await uploadService.appendChunk(req.params.uploadId as string, index, req);
      res.status(204).end();
    }),
  );

  // Lets a client resume after a dropped connection or a reload.
  router.get(
    '/:uploadId',
    asyncHandler(async (req, res) => {
      const status = await uploadService.getStatus(req.params.uploadId as string);
      res.status(200).json(status);
    }),
  );

  router.post(
    '/:uploadId/complete',
    asyncHandler(async (req, res) => {
      const body = parse(completeSchema, req.body ?? {});
      const rec = await uploadService.complete(req.params.uploadId as string, body);
      const shareUrl = `/v/${rec.shareId}`;
      res.status(201).json({ ...rec, shareUrl });
    }),
  );

  return router;
}
