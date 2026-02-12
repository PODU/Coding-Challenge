import { Router } from 'express';
import { asyncHandler } from '../errors';
import { streamBlob } from '../http/stream';
import type { ShareService } from '../services/ShareService';
import type { Storage } from '../storage/Storage';

export function buildShareRouter(shareService: ShareService, storage: Storage): Router {
  const router = Router();

  router.get(
    '/:shareId',
    asyncHandler(async (req, res) => {
      const rec = await shareService.resolve(req.params.shareId as string);
      res.status(200).json(rec);
    }),
  );

  router.get(
    '/:shareId/stream',
    asyncHandler(async (req, res) => {
      const target = await shareService.streamTarget(req.params.shareId as string);
      streamBlob(req, res, storage, target);
    }),
  );

  return router;
}
