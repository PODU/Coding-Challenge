import { ApiError } from '../errors';
import type { RecordingsRepo, ListResult } from '../repo/Recordings';
import type { Storage } from '../storage/Storage';
import type { Recording } from '../../../shared/types';

export interface BlobTarget {
  key: string;
  mimeType: string;
  size: number;
}

export class RecordingService {
  private readonly repo: RecordingsRepo;
  private readonly storage: Storage;

  constructor(repo: RecordingsRepo, storage: Storage) {
    this.repo = repo;
    this.storage = storage;
  }

  async list(opts?: { limit?: number; cursor?: string }): Promise<ListResult> {
    return this.repo.list(opts);
  }

  async get(id: string): Promise<Recording> {
    const rec = await this.repo.getById(id);
    if (!rec) {
      throw ApiError.notFound('Recording not found');
    }
    return rec;
  }

  /** The size comes from storage, not the stored metadata. */
  async streamTarget(id: string): Promise<BlobTarget> {
    const rec = await this.get(id);
    return this.blobTarget(rec);
  }

  async delete(id: string): Promise<void> {
    const rec = await this.repo.getById(id);
    if (!rec) {
      return;
    }
    await this.repo.delete(id);
    await this.storage.delete(rec.filename);
  }

  private async blobTarget(rec: Recording): Promise<BlobTarget> {
    const { sizeBytes } = await this.storage.stat(rec.filename).catch(() => {
      throw ApiError.notFound('Recording blob not found');
    });
    return { key: rec.filename, mimeType: rec.mimeType, size: sizeBytes };
  }
}
