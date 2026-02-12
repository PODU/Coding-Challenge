import express, { type Express, type Request, type Response } from 'express';
import path from 'node:path';
import { existsSync } from 'node:fs';
import { errorHandler } from './errors';
import { buildUploadsRouter } from './routes/uploads';
import { buildRecordingsRouter } from './routes/recordings';
import { buildShareRouter } from './routes/share';
import type { UploadService } from './services/UploadService';
import type { RecordingService } from './services/RecordingService';
import type { ShareService } from './services/ShareService';
import type { Storage } from './storage/Storage';

export interface AppDeps {
  uploadService: UploadService;
  recordingService: RecordingService;
  shareService: ShareService;
  storage: Storage;
  /** Built web client (web/dist), served if present. */
  webDir?: string;
}

export function createApp(deps: AppDeps): Express {
  const app = express();
  app.disable('x-powered-by');

  // Only parses application/json, so octet-stream chunk uploads stay unread streams.
  app.use(express.json({ limit: '1mb' }));

  app.get('/api/health', (_req: Request, res: Response) => {
    res.json({ ok: true });
  });

  app.use('/api/uploads', buildUploadsRouter(deps.uploadService));
  app.use('/api/recordings', buildRecordingsRouter(deps.recordingService, deps.storage));
  app.use('/api/share', buildShareRouter(deps.shareService, deps.storage));

  if (deps.webDir && existsSync(deps.webDir)) {
    const webDir = deps.webDir;
    app.use(express.static(webDir));
    app.get('/', (_req, res) => res.sendFile(path.join(webDir, 'index.html')));
    app.get('/record', (_req, res) => res.sendFile(path.join(webDir, 'record.html')));
    app.get('/v/:shareId', (_req, res) => res.sendFile(path.join(webDir, 'v.html')));
  }

  app.use('/api', (_req: Request, res: Response) => {
    res.status(404).json({ error: { code: 'NOT_FOUND', message: 'Unknown endpoint' } });
  });

  app.use(errorHandler);
  return app;
}
