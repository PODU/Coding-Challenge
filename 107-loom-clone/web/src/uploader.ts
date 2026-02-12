// Uploads MediaRecorder chunks in order as they arrive. On failure it asks the
// server which chunk it expects next, then retries from there.
import { completeUpload, createUpload, getUploadStatus, putChunk } from './api';
import type { CompleteUploadResponse } from '../../shared/types';

export interface UploaderOptions {
  filename: string;
  mimeType: string;
  maxRetries?: number;
}

export class ResumableUploader {
  private readonly opts: UploaderOptions;
  private uploadId: string | null = null;
  private nextIndex = 0;
  private queue: Blob[] = [];
  private pumping = false;
  private failed: Error | null = null;

  constructor(opts: UploaderOptions) {
    this.opts = opts;
  }

  /** Call once before adding chunks. */
  async start(): Promise<void> {
    const { uploadId } = await createUpload({
      filename: this.opts.filename,
      mimeType: this.opts.mimeType,
    });
    this.uploadId = uploadId;
  }

  addChunk(chunk: Blob): void {
    if (chunk.size === 0) {
      return;
    }
    this.queue.push(chunk);
    void this.pump();
  }

  private async pump(): Promise<void> {
    if (this.pumping || this.failed || this.uploadId === null) {
      return;
    }
    this.pumping = true;
    try {
      while (this.queue.length > 0) {
        const chunk = this.queue[0] as Blob;
        await this.sendWithRetry(chunk, this.nextIndex);
        this.queue.shift();
        this.nextIndex += 1;
      }
    } catch (err) {
      this.failed = err instanceof Error ? err : new Error(String(err));
    } finally {
      this.pumping = false;
    }
  }

  private async sendWithRetry(chunk: Blob, index: number): Promise<void> {
    const maxRetries = this.opts.maxRetries ?? 5;
    let attempt = 0;
    for (;;) {
      try {
        await putChunk(this.uploadId as string, index, chunk);
        return;
      } catch (err) {
        attempt += 1;
        if (attempt > maxRetries) {
          throw err;
        }
        // The request may have reached the server even though we saw an error.
        try {
          const status = await getUploadStatus(this.uploadId as string);
          if (status.nextExpectedIndex > index) {
            return; // server already has this chunk
          }
          this.nextIndex = status.nextExpectedIndex;
        } catch {
          // Retry anyway.
        }
        await delay(Math.min(1000 * attempt, 5000));
      }
    }
  }

  /** Wait for queued chunks to upload, then complete the upload. */
  async finish(input: { title?: string; durationMs?: number }): Promise<CompleteUploadResponse> {
    while (this.queue.length > 0 || this.pumping) {
      if (this.failed) {
        throw this.failed;
      }
      await this.pump();
      if (this.queue.length > 0) {
        await delay(50);
      }
    }
    if (this.failed) {
      throw this.failed;
    }
    return completeUpload(this.uploadId as string, input);
  }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
