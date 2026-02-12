import { createReadStream, createWriteStream } from 'node:fs';
import { mkdir, stat, unlink } from 'node:fs/promises';
import path from 'node:path';
import { pipeline } from 'node:stream/promises';
import type { Readable } from 'node:stream';
import type { BlobStat, Storage } from './Storage';
import type { ByteRange } from '../range';

/** One file per blob, all in a single directory. */
export class LocalDiskStorage implements Storage {
  private readonly dir: string;
  private ready: Promise<void> | null = null;

  constructor(dir: string) {
    this.dir = dir;
  }

  private async ensureDir(): Promise<void> {
    if (!this.ready) {
      this.ready = mkdir(this.dir, { recursive: true }).then(() => undefined);
    }
    await this.ready;
  }

  private resolve(key: string): string {
    // Keys are generated server-side, but refuse anything that could escape the dir.
    if (key.includes('/') || key.includes('\\') || key.includes('..')) {
      throw new Error(`Invalid blob key: ${key}`);
    }
    return path.join(this.dir, key);
  }

  async put(key: string, source: Readable): Promise<BlobStat> {
    await this.ensureDir();
    const target = this.resolve(key);
    await pipeline(source, createWriteStream(target));
    return { sizeBytes: (await stat(target)).size };
  }

  getReadStream(key: string, range?: ByteRange): Readable {
    const target = this.resolve(key);
    if (range) {
      return createReadStream(target, { start: range.start, end: range.end });
    }
    return createReadStream(target);
  }

  async stat(key: string): Promise<BlobStat> {
    return { sizeBytes: (await stat(this.resolve(key))).size };
  }

  async delete(key: string): Promise<void> {
    try {
      await unlink(this.resolve(key));
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== 'ENOENT') {
        throw err;
      }
    }
  }

  async exists(key: string): Promise<boolean> {
    try {
      await stat(this.resolve(key));
      return true;
    } catch {
      return false;
    }
  }
}
