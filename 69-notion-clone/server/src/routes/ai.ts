import { Router } from 'express';
import { z } from 'zod';
import { asyncHandler, parse, requireAuth } from '../http.js';
import type { Services } from '../http.js';
import { AppError } from '../errors.js';

const aiSchema = z.object({
  prompt: z.string().min(1).max(8000),
  pageId: z.string().uuid().optional(),
});

/** Passes a prompt to a local Ollama instance. Off unless enabled in config. */
export function aiRouter(services: Services): Router {
  const router = Router();
  router.use(requireAuth(services.auth));

  router.post(
    '/complete',
    asyncHandler(async (req, res) => {
      if (!services.config.aiEnabled) {
        throw AppError.notFound('AI feature is disabled (set AI_ENABLED=true)');
      }
      const { prompt } = parse(aiSchema, req.body);
      const host = process.env.OLLAMA_HOST ?? 'http://localhost:11434';
      const model = process.env.OLLAMA_MODEL ?? 'llama3';
      const r = await fetch(`${host}/api/generate`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ model, prompt, stream: false }),
      });
      if (!r.ok) throw new AppError('INTERNAL', 'AI provider error');
      const data = (await r.json()) as { response?: string };
      res.json({ text: data.response ?? '' });
    }),
  );

  return router;
}
