// Domain types and the shapes the API sends and receives.

export const NOTE_COLORS = [
  'default',
  'red',
  'orange',
  'yellow',
  'green',
  'teal',
  'blue',
  'purple',
  'pink',
  'gray',
] as const;

export type NoteColor = (typeof NOTE_COLORS)[number];

export interface Note {
  id: string;
  title: string;
  content: string;
  color: NoteColor;
  pinned: boolean;
  archived: boolean;
  trashed: boolean;
  position: number;
  labels: string[]; // label names
  createdAt: string;
  updatedAt: string;
}

export interface User {
  id: string;
  provider: 'google' | 'local';
  providerId: string;
  email: string | null;
  displayName: string | null;
  createdAt: string;
}

export interface Label {
  id: string;
  userId: string;
  name: string;
}

export interface NoteRow {
  id: string;
  userId: string;
  title: string;
  content: string;
  color: NoteColor;
  pinned: boolean;
  archived: boolean;
  trashed: boolean;
  position: number;
  createdAt: string;
  updatedAt: string;
}

export interface NoteLabelRow {
  noteId: string;
  labelId: string;
}

export type ErrorCode =
  | 'VALIDATION_ERROR'
  | 'NOT_FOUND'
  | 'UNAUTHENTICATED'
  | 'STORAGE_ERROR'
  | 'INTERNAL_ERROR'
  | 'CONFLICT';

export interface ApiError {
  error: {
    code: ErrorCode;
    message: string;
    details?: unknown;
  };
}
