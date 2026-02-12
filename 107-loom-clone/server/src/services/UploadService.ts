import { createReadStream, createWriteStream } from 'node:fs';
import { mkdir, readFile, writeFile, stat, unlink, readdir } from 'node:fs/promises';
import path from 'node:path';
import type { Readable } from 'node:stream';
import { ApiError } from '../errors';
import { newBlobKey, newId, newShareId } from '../ids';
import type { Storage } from '../storage/Storage';
import type { RecordingsRepo } from '../repo/Recordings';
import type { Recording } from '../../../shared/types';

interface SessionState {
  uploadId: string;
  filename: string;
  mimeType: string;
  totalBytes?: number;
  receivedChunks: number[];
  bytes: number;
  createdAt: string;
  status: 'open' | 'completed';
  recordingId?: string;
}

export interface UploadServiceOptions {
  /** Holds each session's `.part` data and `.json` state. */
  uploadsDir: string;
  storage: Storage;
  repo: RecordingsRepo;
  /** Per-upload size limit. Defaults to 2 GiB. */
  maxBytes?: number;
}

const DEFAULT_MAX_BYTES = 2 * 1024 * 1024 * 1024;

export class UploadService {
  private readonly uploadsDir: string;
  private readonly storage: Storage;
  private readonly repo: RecordingsRepo;
  private readonly maxBytes: number;
  private readonly cache = new Map<string, SessionState>();
  private ready: Promise<void> | null = null;

  constructor(opts: UploadServiceOptions) {
    this.uploadsDir = opts.uploadsDir;
    this.storage = opts.storage;
    this.repo = opts.repo;
    this.maxBytes = opts.maxBytes ?? DEFAULT_MAX_BYTES;
  }

  private async ensureDir(): Promise<void> {
    if (!this.ready) {
      this.ready = mkdir(this.uploadsDir, { recursive: true }).then(() => undefined);
    }
    await this.ready;
  }

  private sessionFile(uploadId: string): string {
    return path.join(this.uploadsDir, `${uploadId}.json`);
  }

  private partFile(uploadId: string): string {
    return path.join(this.uploadsDir, `${uploadId}.part`);
  }

  private async persist(session: SessionState): Promise<void> {
    this.cache.set(session.uploadId, session);
    await writeFile(this.sessionFile(session.uploadId), JSON.stringify(session), 'utf8');
  }

  private async loadSession(uploadId: string): Promise<SessionState> {
    const cached = this.cache.get(uploadId);
    if (cached) {
      return cached;
    }
    try {
      const raw = await readFile(this.sessionFile(uploadId), 'utf8');
      const session = JSON.parse(raw) as SessionState;
      this.cache.set(uploadId, session);
      return session;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
        throw ApiError.notFound('Upload session not found');
      }
      throw err;
    }
  }

  async createSession(input: {
    filename: string;
    mimeType: string;
    totalBytes?: number;
  }): Promise<{ uploadId: string }> {
    await this.ensureDir();
    const uploadId = newId();
    const session: SessionState = {
      uploadId,
      filename: input.filename,
      mimeType: input.mimeType,
      receivedChunks: [],
      bytes: 0,
      createdAt: new Date().toISOString(),
      status: 'open',
    };
    if (input.totalBytes !== undefined) {
      session.totalBytes = input.totalBytes;
    }
    await writeFile(this.partFile(uploadId), Buffer.alloc(0));
    await this.persist(session);
    return { uploadId };
  }

  async getStatus(uploadId: string): Promise<{ receivedChunks: number[]; nextExpectedIndex: number }> {
    const { receivedChunks } = await this.loadSession(uploadId);
    return { receivedChunks, nextExpectedIndex: receivedChunks.length };
  }

  /**
   * Append chunk `index`. Chunks must arrive in order from 0. A chunk we already
   * have is ignored (so retries are safe); skipping ahead is a 409.
   */
  async appendChunk(uploadId: string, index: number, body: Readable): Promise<void> {
    await this.ensureDir();
    const session = await this.loadSession(uploadId);
    if (session.status === 'completed') {
      await drain(body);
      throw ApiError.conflict('UPLOAD_COMPLETED', 'Upload session is already completed');
    }
    const next = session.receivedChunks.length;
    if (index < next) {
      await drain(body);
      return;
    }
    if (index > next) {
      await drain(body);
      throw ApiError.conflict('CHUNK_GAP', `Expected chunk ${next}, got ${index}`);
    }
    session.bytes += await appendToFile(this.partFile(uploadId), body, this.maxBytes - session.bytes);
    session.receivedChunks.push(index);
    await this.persist(session);
  }

  /**
   * Move the assembled upload into storage and create its Recording. Calling it
   * again returns the same Recording.
   */
  async complete(
    uploadId: string,
    input: { title?: string; durationMs?: number },
  ): Promise<Recording> {
    const session = await this.loadSession(uploadId);

    if (session.status === 'completed' && session.recordingId) {
      const existing = await this.repo.getById(session.recordingId);
      if (existing) {
        return existing;
      }
    }

    const partPath = this.partFile(uploadId);
    let size: number;
    try {
      size = (await stat(partPath)).size;
    } catch {
      size = 0;
    }
    if (size === 0) {
      throw ApiError.badRequest('EMPTY_UPLOAD', 'Cannot complete an empty upload');
    }

    const key = newBlobKey();
    const written = await this.storage.put(key, createReadStream(partPath));

    const createdAt = new Date().toISOString();
    const rec: Recording = {
      id: newId(),
      shareId: newShareId(),
      title: input.title?.trim() || defaultTitle(createdAt),
      filename: key,
      mimeType: session.mimeType,
      sizeBytes: written.sizeBytes,
      durationMs: input.durationMs ?? 0,
      createdAt,
    };
    await this.repo.create(rec);

    session.status = 'completed';
    session.recordingId = rec.id;
    await this.persist(session);

    await unlink(partPath).catch(() => undefined);
    return rec;
  }

  /** Remove unfinished sessions older than `ttlMs`. Returns how many went. */
  async sweepExpired(ttlMs: number): Promise<number> {
    await this.ensureDir();
    let removed = 0;
    const now = Date.now();
    let entries: string[];
    try {
      entries = await readdir(this.uploadsDir);
    } catch {
      return 0;
    }
    for (const entry of entries) {
      if (!entry.endsWith('.json')) {
        continue;
      }
      const uploadId = entry.slice(0, -'.json'.length);
      try {
        const session = await this.loadSession(uploadId);
        if (session.status === 'completed') {
          continue;
        }
        if (now - new Date(session.createdAt).getTime() > ttlMs) {
          await unlink(this.sessionFile(uploadId)).catch(() => undefined);
          await unlink(this.partFile(uploadId)).catch(() => undefined);
          this.cache.delete(uploadId);
          removed += 1;
        }
      } catch {
        // Skip sessions we can't read.
      }
    }
    return removed;
  }
}

function defaultTitle(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `Recording ${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(
    d.getHours(),
  )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/**
 * Append `body` to `file` and resolve with the byte count. Past `remaining`
 * bytes we stop writing but keep reading to the end, so the client can still
 * receive the 413.
 */
function appendToFile(file: string, body: Readable, remaining: number): Promise<number> {
  return new Promise<number>((resolve, reject) => {
    const ws = createWriteStream(file, { flags: 'a' });
    let count = 0;
    let over = false;
    let ioError: Error | null = null;

    ws.on('error', (err: Error) => {
      ioError = ioError ?? err;
    });

    body.on('data', (chunk: Buffer) => {
      count += chunk.length;
      if (over) {
        return;
      }
      if (count > remaining) {
        over = true;
        ws.end();
        return;
      }
      if (!ws.write(chunk)) {
        body.pause();
        ws.once('drain', () => body.resume());
      }
    });

    body.on('error', (err: Error) => {
      ioError = ioError ?? err;
    });

    body.on('end', () => {
      if (over) {
        ws.destroy();
        reject(ApiError.payloadTooLarge('Upload exceeds maximum allowed size'));
        return;
      }
      ws.end(() => {
        if (ioError) {
          reject(ioError);
        } else {
          resolve(count);
        }
      });
    });
  });
}

/** Read and discard a request body we're not going to use. */
function drain(body: Readable): Promise<void> {
  return new Promise<void>((resolve) => {
    let done = false;
    const finish = (): void => {
      if (!done) {
        done = true;
        resolve();
      }
    };
    body.on('end', finish);
    body.on('close', finish);
    body.on('error', finish);
    body.resume();
  });
}
