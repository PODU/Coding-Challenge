import type { Request, Response } from 'express';
import { parseRange } from '../range';
import type { Storage } from '../storage/Storage';
import type { BlobTarget } from '../services/RecordingService';

/** Serve a blob, honouring Range requests so <video> can seek. */
export function streamBlob(req: Request, res: Response, storage: Storage, target: BlobTarget): void {
  const { size, mimeType, key } = target;
  const range = parseRange(req.headers.range, size);

  res.setHeader('Accept-Ranges', 'bytes');
  res.setHeader('Content-Type', mimeType);
  res.setHeader('Cache-Control', 'private, max-age=0, must-revalidate');

  if (range === 'invalid') {
    res.status(416).setHeader('Content-Range', `bytes */${size}`);
    res.end();
    return;
  }

  if (range === null) {
    res.status(200);
    res.setHeader('Content-Length', String(size));
    if (req.method === 'HEAD') {
      res.end();
      return;
    }
    const stream = storage.getReadStream(key);
    stream.on('error', () => res.destroy());
    stream.pipe(res);
    return;
  }

  const length = range.end - range.start + 1;
  res.status(206);
  res.setHeader('Content-Range', `bytes ${range.start}-${range.end}/${size}`);
  res.setHeader('Content-Length', String(length));
  if (req.method === 'HEAD') {
    res.end();
    return;
  }
  const stream = storage.getReadStream(key, range);
  stream.on('error', () => res.destroy());
  stream.pipe(res);
}
