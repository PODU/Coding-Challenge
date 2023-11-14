import { Router } from 'express';
import { z } from 'zod';
import { asyncHandler, notFound } from '../middleware/errors';
import { requireAuth } from '../middleware/auth';

const createSchema = z.object({ name: z.string().trim().min(1, 'name is required').max(100) });

export function labelsRouter(): Router {
  const router = Router();
  router.use(requireAuth);

  router.get(
    '/',
    asyncHandler((req, res) => {
      res.json({ labels: req.repos!.labels.listForUser(req.userId!) });
    }),
  );

  router.post(
    '/',
    asyncHandler((req, res) => {
      const { name } = createSchema.parse(req.body ?? {});
      const label = req.repos!.labels.create(req.userId!, name);
      res.status(201).json({ label });
    }),
  );

  router.delete(
    '/:id',
    asyncHandler((req, res) => {
      const ok = req.repos!.labels.delete(req.userId!, req.params.id);
      if (!ok) throw notFound('Label not found');
      res.json({ ok: true });
    }),
  );

  return router;
}
