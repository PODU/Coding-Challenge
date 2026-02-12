// The /v/:shareId page.
import { resolveShare, shareStreamUrl } from './api';

function shareIdFromPath(): string | null {
  const m = /^\/v\/([^/]+)\/?$/.exec(location.pathname);
  return m ? decodeURIComponent(m[1] as string) : null;
}

async function main(): Promise<void> {
  const titleEl = document.getElementById('title');
  const metaEl = document.getElementById('meta');
  const video = document.getElementById('player') as HTMLVideoElement | null;
  const errorEl = document.getElementById('error');
  if (!video || !titleEl || !metaEl || !errorEl) {
    return;
  }

  const shareId = shareIdFromPath();
  if (!shareId) {
    errorEl.textContent = 'Invalid share link.';
    return;
  }

  try {
    const rec = await resolveShare(shareId);
    titleEl.textContent = rec.title;
    document.title = `${rec.title} · Loom clone`;
    const created = new Date(rec.createdAt).toLocaleString();
    metaEl.textContent = `${created} · ${(rec.sizeBytes / (1024 * 1024)).toFixed(1)} MB`;
    video.src = shareStreamUrl(shareId);
    video.setAttribute('controls', '');
  } catch (err) {
    errorEl.textContent =
      (err as Error).message === 'Shared recording not found'
        ? 'This recording does not exist or has been deleted.'
        : `Could not load recording: ${(err as Error).message}`;
  }
}

void main();
