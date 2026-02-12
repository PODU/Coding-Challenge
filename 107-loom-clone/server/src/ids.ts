import { randomBytes, randomUUID } from 'node:crypto';

export function newId(): string {
  return randomUUID();
}

/** 128 random bits as base64url (22 chars). Anyone with the link can view. */
export function newShareId(): string {
  return randomBytes(16).toString('base64url');
}

/** Blob keys are always generated here, never taken from the client. */
export function newBlobKey(): string {
  return randomUUID();
}
