import { z } from 'zod';
import { config } from '../config';
import { LANGUAGES, type Language } from '../types';

const languageSchema = z.enum(LANGUAGES as unknown as [Language, ...Language[]]);

const sourceSchema = z
  .string({ required_error: 'source is required' })
  .min(1, 'source must not be empty')
  .refine(
    (s) => Buffer.byteLength(s, 'utf8') <= config.maxSourceBytes,
    { message: `source exceeds ${config.maxSourceBytes} byte limit` },
  );

const stdinSchema = z
  .string()
  .refine((s) => Buffer.byteLength(s, 'utf8') <= config.maxStdinBytes, {
    message: `stdin exceeds ${config.maxStdinBytes} byte limit`,
  })
  .optional();

export const runRequestSchema = z.object({
  language: languageSchema,
  source: sourceSchema,
  stdin: stdinSchema,
  timeoutMs: z.number().int().positive().optional(),
});

export const submitRequestSchema = z.object({
  language: languageSchema,
  source: sourceSchema,
});

export type RunRequestBody = z.infer<typeof runRequestSchema>;
export type SubmitRequestBody = z.infer<typeof submitRequestSchema>;

/** Distinguishes "too big" (413) from ordinary validation failures (400). */
export function isSizeViolation(error: z.ZodError): boolean {
  return error.issues.some((i) => /byte limit/.test(i.message));
}
