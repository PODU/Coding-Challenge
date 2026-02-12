import { Readable } from 'node:stream';
import type { BlobStat, Storage } from './Storage';
import type { ByteRange } from '../range';

/**
 * In-memory storage for tests. It counts the chunks each `put` receives so tests
 * can check that uploads are streamed rather than buffered whole.
 */
export class MockStorage implements Storage {
  private readonly blobs = new Map<string, Buffer>();

  streamingPutUsed = false;
  /** Chunks seen across all puts. */
  observedChunkCount = 0;

  async put(key: string, source: Readable): Promise<BlobStat> {
    const chunks: Buffer[] = [];
    for await (const chunk of source) {
      this.observedChunkCount += 1;
      if (Buffer.isBuffer(chunk)) {
        chunks.push(chunk);
      } else if (typeof chunk === 'string') {
        chunks.push(Buffer.from(chunk));
      } else {
        chunks.push(Buffer.from(chunk as Uint8Array));
      }
    }
    this.streamingPutUsed = true;
    const buf = Buffer.concat(chunks);
    this.blobs.set(key, buf);
    return { sizeBytes: buf.length };
  }

  getReadStream(key: string, range?: ByteRange): Readable {
    const buf = this.blobs.get(key);
    if (!buf) {
      // Fail on the stream, the way a missing file does on disk.
      return new Readable({
        read() {
          this.destroy(new Error(`Blob not found: ${key}`));
        },
      });
    }
    const slice = range ? buf.subarray(range.start, range.end + 1) : buf;
    return Readable.from(slice);
  }

  async stat(key: string): Promise<BlobStat> {
    const buf = this.blobs.get(key);
    if (!buf) {
      throw new Error(`Blob not found: ${key}`);
    }
    return { sizeBytes: buf.length };
  }

  async delete(key: string): Promise<void> {
    this.blobs.delete(key);
  }

  async exists(key: string): Promise<boolean> {
    return this.blobs.has(key);
  }

  read(key: string): Buffer | undefined {
    return this.blobs.get(key);
  }
}
