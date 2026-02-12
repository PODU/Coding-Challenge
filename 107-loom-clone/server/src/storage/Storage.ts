import type { Readable } from 'node:stream';
import type { ByteRange } from '../range';

export interface BlobStat {
  sizeBytes: number;
}

/** Blob storage. Implementations should stream and never hold a whole blob in memory. */
export interface Storage {
  put(key: string, source: Readable): Promise<BlobStat>;
  /** `range` is inclusive. */
  getReadStream(key: string, range?: ByteRange): Readable;
  /** Throws if the blob is missing. */
  stat(key: string): Promise<BlobStat>;
  /** Deleting a missing blob is not an error. */
  delete(key: string): Promise<void>;
  exists(key: string): Promise<boolean>;
}
