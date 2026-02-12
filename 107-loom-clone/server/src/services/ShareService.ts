import { ApiError } from '../errors';
import type { RecordingsRepo } from '../repo/Recordings';
import type { Storage } from '../storage/Storage';
import type { PublicRecording, Recording } from '../../../shared/types';
import type { BlobTarget } from './RecordingService';

/** Strips the internal id and blob key before a recording goes public. */
export function toPublic(rec: Recording): PublicRecording {
  return {
    shareId: rec.shareId,
    title: rec.title,
    mimeType: rec.mimeType,
    sizeBytes: rec.sizeBytes,
    durationMs: rec.durationMs,
    createdAt: rec.createdAt,
  };
}

export class ShareService {
  private readonly repo: RecordingsRepo;
  private readonly storage: Storage;

  constructor(repo: RecordingsRepo, storage: Storage) {
    this.repo = repo;
    this.storage = storage;
  }

  async resolve(shareId: string): Promise<PublicRecording> {
    const rec = await this.repo.getByShareId(shareId);
    if (!rec) {
      throw ApiError.notFound('Shared recording not found');
    }
    return toPublic(rec);
  }

  async streamTarget(shareId: string): Promise<BlobTarget> {
    const rec = await this.repo.getByShareId(shareId);
    if (!rec) {
      throw ApiError.notFound('Shared recording not found');
    }
    const { sizeBytes } = await this.storage.stat(rec.filename).catch(() => {
      throw ApiError.notFound('Recording blob not found');
    });
    return { key: rec.filename, mimeType: rec.mimeType, size: sizeBytes };
  }
}
