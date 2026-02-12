import { deleteRecording, listRecordings } from './api';
import type { Recording } from '../../shared/types';

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function fmtDuration(ms: number): string {
  const s = Math.round(ms / 1000);
  const m = Math.floor(s / 60);
  return `${m}:${String(s % 60).padStart(2, '0')}`;
}

function render(list: HTMLElement, items: Recording[]): void {
  list.innerHTML = '';
  if (items.length === 0) {
    list.innerHTML = '<p class="empty">No recordings yet. Click “Record” to make one.</p>';
    return;
  }
  for (const rec of items) {
    const card = document.createElement('div');
    card.className = 'card';
    const created = new Date(rec.createdAt).toLocaleString();
    card.innerHTML = `
      <div class="card-body">
        <h3>${escapeHtml(rec.title)}</h3>
        <p class="meta">${created} · ${fmtDuration(rec.durationMs)} · ${fmtBytes(rec.sizeBytes)}</p>
        <div class="actions">
          <a class="btn" href="/v/${rec.shareId}">Open</a>
          <button class="btn copy" data-share="${rec.shareId}">Copy link</button>
          <button class="btn danger del" data-id="${rec.id}">Delete</button>
        </div>
      </div>`;
    list.appendChild(card);
  }

  list.querySelectorAll<HTMLButtonElement>('.copy').forEach((btn) => {
    btn.addEventListener('click', () => {
      const link = `${location.origin}/v/${btn.dataset.share ?? ''}`;
      void navigator.clipboard?.writeText(link);
      btn.textContent = 'Copied!';
      setTimeout(() => (btn.textContent = 'Copy link'), 1500);
    });
  });
  list.querySelectorAll<HTMLButtonElement>('.del').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.id;
      if (!id || !confirm('Delete this recording?')) return;
      await deleteRecording(id);
      await load();
    });
  });
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] ?? c,
  );
}

async function load(): Promise<void> {
  const list = document.getElementById('list');
  if (!list) return;
  try {
    const res = await listRecordings(50);
    render(list, res.items);
  } catch (err) {
    list.innerHTML = `<p class="error">Failed to load: ${escapeHtml((err as Error).message)}</p>`;
  }
}

void load();
