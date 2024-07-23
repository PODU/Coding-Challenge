/** Request validation schemas. */
import { z } from 'zod';
import { BLOCK_TYPES } from '../../../shared/types.js';

export const blockTypeSchema = z.enum(BLOCK_TYPES);

export const markSchema = z.object({
  start: z.number().int().nonnegative(),
  end: z.number().int().nonnegative(),
  bold: z.boolean().optional(),
  italic: z.boolean().optional(),
});

export const blockPropsSchema = z
  .object({ checked: z.boolean().optional() })
  .strict();

export const registerSchema = z.object({
  email: z.string().email(),
  password: z.string().min(6).max(200),
});

export const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

export const createPageSchema = z.object({
  title: z.string().max(500).optional(),
  parentPageId: z.string().uuid().nullable().optional(),
});

export const updatePageSchema = z
  .object({
    title: z.string().max(500).optional(),
    parentPageId: z.string().uuid().nullable().optional(),
    position: z.number().optional(),
  })
  .refine((v) => Object.keys(v).length > 0, {
    message: 'At least one field is required',
  });

export const reorderPageSchema = z.object({
  afterPageId: z.string().uuid().nullable().optional(),
  parentPageId: z.string().uuid().nullable().optional(),
});

export const createBlockSchema = z.object({
  type: blockTypeSchema,
  text: z.string().max(100_000).optional(),
  marks: z.array(markSchema).optional(),
  props: blockPropsSchema.optional(),
  parentId: z.string().uuid().nullable().optional(),
  afterBlockId: z.string().uuid().nullable().optional(),
});

export const updateBlockSchema = z
  .object({
    text: z.string().max(100_000).optional(),
    marks: z.array(markSchema).optional(),
    props: blockPropsSchema.optional(),
  })
  .refine((v) => Object.keys(v).length > 0, {
    message: 'At least one field is required',
  });

export const changeTypeSchema = z.object({ type: blockTypeSchema });

export const moveBlockSchema = z.object({
  parentId: z.string().uuid().nullable().optional(),
  afterBlockId: z.string().uuid().nullable().optional(),
});

export const batchSchema = z.object({
  ops: z.array(z.record(z.unknown())).max(500),
});

export type RegisterInput = z.infer<typeof registerSchema>;
export type LoginInput = z.infer<typeof loginSchema>;
export type CreatePageInput = z.infer<typeof createPageSchema>;
export type UpdatePageInput = z.infer<typeof updatePageSchema>;
export type ReorderPageInput = z.infer<typeof reorderPageSchema>;
export type CreateBlockInput = z.infer<typeof createBlockSchema>;
export type UpdateBlockInput = z.infer<typeof updateBlockSchema>;
export type ChangeTypeInput = z.infer<typeof changeTypeSchema>;
export type MoveBlockInput = z.infer<typeof moveBlockSchema>;
