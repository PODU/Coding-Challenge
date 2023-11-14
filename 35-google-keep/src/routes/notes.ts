import { Router } from 'express';
import { z } from 'zod';
import { NOTE_COLORS } from '../types';
import { asyncHandler, notFound, validationError } from '../middleware/errors';
import { requireAuth } from '../middleware/auth';

const MAX_TEXT = 10_000;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const text = z.string().max(MAX_TEXT, `must be at most ${MAX_TEXT} characters`);
const color = z.enum(NOTE_COLORS);
const labels = z.array(z.string().trim().min(1, 'label must be non-empty')).max(50);

const createSchema = z
  .object({
    title: text.optional(),
    content: text.optional(),
    color: color.optional(),
    pinned: z.boolean().optional(),
    archived: z.boolean().optional(),
    labels: labels.optional(),
  })
  .strict()
  .refine((v) => (v.title?.trim() || v.content?.trim()), {
    message: 'a note must have a title or content',
    path: ['title'],
  });

const updateSchema = z
  .object({
    title: text.optional(),
    content: text.optional(),
    color: color.optional(),
    pinned: z.boolean().optional(),
    archived: z.boolean().optional(),
    position: z.number().finite().optional(),
    labels: labels.optional(),
  })
  .strict();

function bool(v: unknown): boolean | undefined {
  if (v === undefined) return undefined;
  if (v === '1' || v === 'true' || v === true) return true;
  if (v === '0' || v === 'false' || v === false) return false;
  return undefined;
}

export function notesRouter(): Router {
  const router = Router();
  router.use(requireAuth);

  // Supports search, filters and sorting.
  router.get(
    '/',
    asyncHandler((req, res) => {
      const userId = req.userId!;
      const q = req.query;
      const notes = req.repos!.notes.list(userId, {
        q: typeof q.q === 'string' && q.q.trim() ? q.q.trim() : undefined,
        label: typeof q.label === 'string' && q.label ? q.label : undefined,
        archived: bool(q.archived),
        trashed: bool(q.trashed),
        pinned: bool(q.pinned),
        sort:
          q.sort === 'created' || q.sort === 'updated' || q.sort === 'default'
            ? q.sort
            : undefined,
        limit: q.limit ? Number(q.limit) : undefined,
        offset: q.offset ? Number(q.offset) : undefined,
      });
      res.json({ notes });
    }),
  );

  router.get(
    '/:id',
    asyncHandler((req, res) => {
      const { id } = req.params;
      if (!UUID_RE.test(id)) throw notFound('Note not found');
      const note = req.repos!.notes.get(req.userId!, id);
      if (!note) throw notFound('Note not found');
      res.json({ note });
    }),
  );

  router.post(
    '/',
    asyncHandler((req, res) => {
      const input = createSchema.parse(req.body ?? {});
      const note = req.repos!.notes.create(req.userId!, input);
      res.status(201).json({ note });
    }),
  );

  router.patch(
    '/:id',
    asyncHandler((req, res) => {
      const { id } = req.params;
      if (!UUID_RE.test(id)) throw notFound('Note not found');
      const patch = updateSchema.parse(req.body ?? {});
      if (Object.keys(patch).length === 0) throw validationError('no fields to update');
      const note = req.repos!.notes.update(req.userId!, id, patch);
      if (!note) throw notFound('Note not found');
      res.json({ note });
    }),
  );

  // Delete: soft-delete by default, hard-delete with ?purge=1.
  router.delete(
    '/:id',
    asyncHandler((req, res) => {
      const { id } = req.params;
      if (!UUID_RE.test(id)) throw notFound('Note not found');
      const userId = req.userId!;
      if (bool(req.query.purge)) {
        const ok = req.repos!.notes.purge(userId, id);
        if (!ok) throw notFound('Note not found');
        res.json({ ok: true, purged: true });
        return;
      }
      const note = req.repos!.notes.softDelete(userId, id);
      if (!note) throw notFound('Note not found');
      res.json({ note });
    }),
  );

  // Restore from trash / archive.
  router.post(
    '/:id/restore',
    asyncHandler((req, res) => {
      const { id } = req.params;
      if (!UUID_RE.test(id)) throw notFound('Note not found');
      const note = req.repos!.notes.restore(req.userId!, id);
      if (!note) throw notFound('Note not found');
      res.json({ note });
    }),
  );

  return router;
}
