import { Router } from 'express';
import { asyncHandler, getUserId, parse, requireAuth } from '../http.js';
import type { Services } from '../http.js';
import {
  createPageSchema,
  reorderPageSchema,
  updatePageSchema,
} from '../domain/schemas.js';

export function pagesRouter(services: Services): Router {
  const router = Router();
  router.use(requireAuth(services.auth));

  router.get(
    '/',
    asyncHandler((req, res) => {
      res.json(services.pages.list(getUserId(req)));
    }),
  );

  router.post(
    '/',
    asyncHandler((req, res) => {
      const input = parse(createPageSchema, req.body);
      res.status(201).json(services.pages.create(getUserId(req), input));
    }),
  );

  router.get(
    '/:id',
    asyncHandler((req, res) => {
      res.json(services.pages.get(getUserId(req), req.params.id));
    }),
  );

  router.patch(
    '/:id',
    asyncHandler((req, res) => {
      const input = parse(updatePageSchema, req.body);
      res.json(services.pages.update(getUserId(req), req.params.id, input));
    }),
  );

  router.post(
    '/:id/duplicate',
    asyncHandler((req, res) => {
      res.status(201).json(services.pages.duplicate(getUserId(req), req.params.id));
    }),
  );

  router.post(
    '/:id/reorder',
    asyncHandler((req, res) => {
      const input = parse(reorderPageSchema, req.body);
      res.json(services.pages.reorder(getUserId(req), req.params.id, input));
    }),
  );

  router.delete(
    '/:id',
    asyncHandler((req, res) => {
      services.pages.delete(getUserId(req), req.params.id);
      res.status(204).end();
    }),
  );

  return router;
}
