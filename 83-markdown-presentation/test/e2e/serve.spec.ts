import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { mkdtempSync, writeFileSync, rmSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { get } from 'node:http';
import { WebSocket } from 'ws';
import { startServer, type ServerHandle } from '../../src/shell/server.js';

function httpGet(url: string): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    get(url, (res) => {
      let data = '';
      res.on('data', (c) => (data += c));
      res.on('end', () => resolve({ status: res.statusCode ?? 0, body: data }));
    }).on('error', reject);
  });
}

describe('serve — HTTP + live reload (integration)', () => {
  let dir: string;
  let slidesDir: string;
  let handle: ServerHandle;

  beforeAll(async () => {
    dir = mkdtempSync(join(tmpdir(), 'mdp-serve-'));
    slidesDir = join(dir, 'slides');
    mkdirSync(slidesDir, { recursive: true });
    mkdirSync(join(dir, 'images'), { recursive: true });
    writeFileSync(join(dir, 'images', 'pic.txt'), 'asset-bytes');
    writeFileSync(join(slidesDir, 'deck.md'), '# One\n\n---\n\n# Two\n\n---\n\n# Three\n');

    handle = await startServer({ source: slidesDir, port: 0, assetsRoot: dir });
  });

  afterAll(async () => {
    await handle.close();
    rmSync(dir, { recursive: true, force: true });
  });

  it('serves the rendered deck at / with the expected slide count', async () => {
    const res = await httpGet(handle.url);
    expect(res.status).toBe(200);
    expect(res.body).toContain('data-count="3"');
    expect(res.body).toContain('id="slide-1"');
    expect(res.body).toContain('id="slide-3"');
    expect(res.body).toContain('__MDP_LIVERELOAD__'); // live-reload wiring injected
  });

  it('serves the raw Markdown view at /raw', async () => {
    const res = await httpGet(handle.url + 'raw');
    expect(res.status).toBe(200);
    expect(res.body).toContain('# One');
    expect(res.body).toContain('raw');
  });

  it('serves static assets under /assets', async () => {
    const res = await httpGet(handle.url + 'assets/images/pic.txt');
    expect(res.status).toBe(200);
    expect(res.body).toBe('asset-bytes');
  });

  it('returns 404 for unknown paths', async () => {
    const res = await httpGet(handle.url + 'nope');
    expect(res.status).toBe(404);
  });

  it('pushes a {"type":"reload"} message over WebSocket when the source changes', async () => {
    const wsUrl = handle.url.replace('http://', 'ws://') + '__livereload';
    const ws = new WebSocket(wsUrl);

    const reload = new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('no reload message within timeout')), 10000);
      ws.on('message', (data) => {
        clearTimeout(timer);
        resolve(data.toString());
      });
      ws.on('error', reject);
    });

    await new Promise<void>((resolve) => ws.on('open', () => resolve()));
    // Give chokidar a beat to settle before we mutate the file.
    await new Promise((r) => setTimeout(r, 300));
    writeFileSync(join(slidesDir, 'deck.md'), '# Changed\n');

    const msg = await reload;
    expect(JSON.parse(msg)).toEqual({ type: 'reload' });
    ws.close();
  }, 15000);
});
