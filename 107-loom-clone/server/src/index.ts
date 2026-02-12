import path from 'node:path';
import { createApp } from './app';
import { LocalDiskStorage } from './storage/LocalDiskStorage';
import { JsonRecordingsRepo } from './repo/Recordings';
import { UploadService } from './services/UploadService';
import { RecordingService } from './services/RecordingService';
import { ShareService } from './services/ShareService';

async function main(): Promise<void> {
  const port = Number.parseInt(process.env.PORT ?? '3000', 10);
  const dataDir = process.env.DATA_DIR ?? path.resolve(process.cwd(), 'data');
  const blobsDir = path.join(dataDir, 'blobs');
  const uploadsDir = path.join(dataDir, 'uploads');
  const metadataFile = path.join(dataDir, 'metadata.json');
  const webDir = path.resolve(process.cwd(), 'web', 'dist');
  const maxBytes = Number.parseInt(process.env.MAX_UPLOAD_BYTES ?? String(2 * 1024 * 1024 * 1024), 10);

  const storage = new LocalDiskStorage(blobsDir);
  const repo = new JsonRecordingsRepo(metadataFile);
  await repo.load();

  const uploadService = new UploadService({ uploadsDir, storage, repo, maxBytes });
  const recordingService = new RecordingService(repo, storage);
  const shareService = new ShareService(repo, storage);

  // Hourly, drop upload sessions abandoned for more than a day.
  const ttlMs = 24 * 60 * 60 * 1000;
  const sweep = setInterval(() => {
    uploadService.sweepExpired(ttlMs).catch(() => undefined);
  }, 60 * 60 * 1000);
  sweep.unref();

  const app = createApp({ uploadService, recordingService, shareService, storage, webDir });

  app.listen(port, () => {
    // eslint-disable-next-line no-console
    console.log(`Loom-clone backend listening on http://localhost:${port}`);
    // eslint-disable-next-line no-console
    console.log(`Data dir: ${dataDir}`);
  });
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error('Fatal startup error:', err);
  process.exit(1);
});
