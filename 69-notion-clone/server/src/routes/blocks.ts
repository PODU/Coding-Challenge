import { Router } from 'express';
import { asyncHandler, getUserId, parse, requireAuth } from '../http.js';
import type { Services } from '../http.js';
import {
  batchSchema,
  changeTypeSchema,
  createBlockSchema,
  moveBlockSchema,
  updateBlockSchema,
} from '../domain/schemas.js';

/** Page-scoped block routes, mounted at /api/pages. */
export function pageBlocksRouter(services: Services): Router {
  const router = Router();
  router.use(requireAuth(services.auth));

  router.post(
    '/:pageId/blocks',
    asyncHandler((req, res) => {
      const input = parse(createBlockSchema, req.body);
      res
        .status(201)
        .json(services.blocks.create(getUserId(req), req.params.pageId, input));
    }),
  );

  router.post(
    '/:pageId/blocks/batch',
    asyncHandler((req, res) => {
      const { ops } = parse(batchSchema, req.body);
      const blocks = services.blocks.batch(getUserId(req), req.params.pageId, ops);
      res.json({ blocks });
    }),
  );

  return router;
}

/** Block-by-id routes, mounted at /api/blocks. */
export function blocksRouter(services: Services): Router {
  const router = Router();
  router.use(requireAuth(services.auth));

  router.patch(
    '/:id',
    asyncHandler((req, res) => {
      const input = parse(updateBlockSchema, req.body);
      res.json(services.blocks.update(getUserId(req), req.params.id, input));
    }),
  );

  router.post(
    '/:id/type',
    asyncHandler((req, res) => {
      const { type } = parse(changeTypeSchema, req.body);
      res.json(services.blocks.changeType(getUserId(req), req.params.id, type));
    }),
  );

  router.post(
    '/:id/move',
    asyncHandler((req, res) => {
      const input = parse(moveBlockSchema, req.body);
      res.json(services.blocks.move(getUserId(req), req.params.id, input));
    }),
  );

  router.delete(
    '/:id',
    asyncHandler((req, res) => {
      services.blocks.delete(getUserId(req), req.params.id);
      res.status(204).end();
    }),
  );

  return router;
}
