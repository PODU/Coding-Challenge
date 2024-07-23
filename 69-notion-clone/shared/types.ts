/**
 * Types shared by the server and the web client, i.e. the API's shapes.
 */

export const BLOCK_TYPES = [
  'paragraph',
  'heading_1',
  'heading_2',
  'heading_3',
  'bulleted_list_item',
  'numbered_list_item',
  'to_do',
  'quote',
  'divider',
] as const;

export type BlockType = (typeof BLOCK_TYPES)[number];

/** Inline rich-text range (bold/italic), clamped to [0, text.length]. */
export interface Mark {
  start: number;
  end: number;
  bold?: boolean;
  italic?: boolean;
}

/** Type-specific properties. Only `checked` is used today (to_do). */
export interface BlockProps {
  checked?: boolean;
}

export interface Block {
  id: string;
  pageId: string;
  parentId: string | null;
  type: BlockType;
  text: string;
  marks: Mark[];
  props: BlockProps;
  position: number;
  createdAt: string;
  updatedAt: string;
}

/** A block with its children assembled into a tree (API read shape). */
export interface BlockNode extends Block {
  children: BlockNode[];
}

export interface Page {
  id: string;
  userId: string;
  parentPageId: string | null;
  title: string;
  position: number;
  createdAt: string;
  updatedAt: string;
}

export interface User {
  id: string;
  email: string;
  createdAt: string;
}

export interface AuthResponse {
  user: User;
  token: string;
}

export interface PageDetail {
  page: Page;
  blocks: BlockNode[];
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    details?: unknown[];
  };
}
