import { Router } from 'express';
import { z } from 'zod';
import { ApiError, asyncHandler } from '../errors';
import { streamBlob } from '../http/stream';
import type { RecordingService } from '../services/RecordingService';
import type { Storage } from '../storage/Storage';

const listSchema = z.object({
  limit: z.coerce.number().int().positive().max(100).optional(),
  cursor: z.string().optional(),
});

export function buildRecordingsRouter(
  recordingService: RecordingService,
  storage: Storage,
): Router {
  const router = Router();

  router.get(
    '/',
    asyncHandler(async (req, res) => {
      const result = listSchema.safeParse(req.query);
      if (!result.success) {
        throw ApiError.badRequest('VALIDATION', 'Invalid list parameters');
      }
      const list = await recordingService.list(result.data);
      res.status(200).json(list);
    }),
  );

  router.get(
    '/:id',
    asyncHandler(async (req, res) => {
      const rec = await recordingService.get(req.params.id as string);
      res.status(200).json(rec);
    }),
  );

  router.get(
    '/:id/stream',
    asyncHandler(async (req, res) => {
      const target = await recordingService.streamTarget(req.params.id as string);
      streamBlob(req, res, storage, target);
    }),
  );

  router.delete(
    '/:id',
    asyncHandler(async (req, res) => {
      await recordingService.delete(req.params.id as string);
      res.status(204).end();
    }),
  );

  return router;
}
