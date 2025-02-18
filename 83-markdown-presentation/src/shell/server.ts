/** Dev server: the deck, a raw Markdown view, assets, and live reload over WebSocket. */
import { createServer, type Server, type IncomingMessage, type ServerResponse } from 'node:http';
import { readFileSync, existsSync, statSync, realpathSync } from 'node:fs';
import { join, normalize, resolve, extname } from 'node:path';
import { WebSocketServer, type WebSocket } from 'ws';
import { renderDeckHtml } from '../core/renderer.js';
import { loadDeck, readSource } from './pipeline.js';
import { loadTemplate } from './template-loader.js';
import { watchPaths, RELOAD_MESSAGE, type Watcher } from './live-reload.js';

const LIVERELOAD_PATH = '/__livereload';

export interface ServeOptions {
  source: string;
  port?: number;
  templateDir?: string;
  /** Directory served under `/assets/`. Defaults to the cwd. */
  assetsRoot?: string;
  /** More paths to watch for live reload, such as the template directory. */
  watch?: string[];
}

export interface ServerHandle {
  server: Server;
  port: number;
  url: string;
  close(): Promise<void>;
}

const MIME: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.json': 'application/json; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
};

function liveReloadSnippet(): string {
  return `<script>window.__MDP_LIVERELOAD__ = (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '${LIVERELOAD_PATH}';</script>`;
}

function isInside(root: string, target: string): boolean {
  return target === root || target.startsWith(root + (process.platform === 'win32' ? '\\' : '/'));
}

/** Resolve a URL path under `root`, or return null if it escapes `root`. */
function safeJoin(root: string, urlPath: string): string | null {
  const target = normalize(join(root, decodeURIComponent(urlPath)));
  return isInside(resolve(root), target) ? target : null;
}

/** Start the dev server. Resolves once it is listening. */
export function startServer(options: ServeOptions): Promise<ServerHandle> {
  const port = options.port ?? 3000;
  const assetsRoot = options.assetsRoot ?? process.cwd();
  const template = loadTemplate(options.templateDir);

  const renderHtml = (): string => {
    const deck = loadDeck(options.source, { assetPrefix: '/assets/' });
    return renderDeckHtml(deck, template, { headExtra: liveReloadSnippet() });
  };

  const handler = (req: IncomingMessage, res: ServerResponse): void => {
    const url = (req.url ?? '/').split('?')[0]!;

    try {
      if (url === '/' || url === '/index.html') {
        const html = renderHtml();
        res.writeHead(200, { 'content-type': MIME['.html']! });
        res.end(html);
        return;
      }

      if (url === '/raw') {
        const raw = readSource(options.source);
        const page =
          `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Raw source</title>` +
          liveReloadSnippet() +
          `<script>${readClientJsForRaw()}</script>` +
          `</head><body><pre id="raw"></pre><script>` +
          `document.getElementById('raw').textContent = ${JSON.stringify(raw)};</script></body></html>`;
        res.writeHead(200, { 'content-type': MIME['.html']! });
        res.end(page);
        return;
      }

      if (url.startsWith('/assets/')) {
        serveStatic(assetsRoot, url.slice('/assets'.length), res);
        return;
      }

      res.writeHead(404, { 'content-type': MIME['.txt']! });
      res.end('Not found');
    } catch (err) {
      res.writeHead(500, { 'content-type': MIME['.txt']! });
      res.end('Internal error: ' + (err instanceof Error ? err.message : String(err)));
    }
  };

  const server = createServer(handler);

  const wss = new WebSocketServer({ noServer: true });
  const clients = new Set<WebSocket>();
  server.on('upgrade', (req, socket, head) => {
    if ((req.url ?? '').split('?')[0] === LIVERELOAD_PATH) {
      wss.handleUpgrade(req, socket, head, (ws) => {
        clients.add(ws);
        ws.on('close', () => clients.delete(ws));
      });
    } else {
      socket.destroy();
    }
  });

  const broadcastReload = (): void => {
    for (const ws of clients) {
      try {
        ws.send(RELOAD_MESSAGE);
      } catch {
        // A dead socket will be cleaned up by its close handler.
      }
    }
  };

  const watchTargets = [options.source, ...(options.watch ?? [])].filter(Boolean);
  let watcher: Watcher | null = null;
  try {
    watcher = watchPaths(watchTargets, broadcastReload);
  } catch {
    // Serve without live reload.
  }

  return new Promise<ServerHandle>((resolvePromise, reject) => {
    server.once('error', reject);
    server.listen(port, () => {
      server.off('error', reject);
      const addr = server.address();
      const actualPort = typeof addr === 'object' && addr ? addr.port : port;
      resolvePromise({
        server,
        port: actualPort,
        url: `http://localhost:${actualPort}/`,
        async close(): Promise<void> {
          if (watcher) await watcher.close();
          for (const ws of clients) ws.close();
          wss.close();
          await new Promise<void>((done) => server.close(() => done()));
        },
      });
    });
  });
}

function readClientJsForRaw(): string {
  // The raw view only needs live reload. The default client does nothing else when there
  // are no slides, so reuse it.
  try {
    return loadTemplate().clientJs;
  } catch {
    return '';
  }
}

function serveStatic(root: string, relPath: string, res: ServerResponse): void {
  const target = safeJoin(root, relPath);
  if (!target || !existsSync(target) || !statSync(target).isFile()) {
    res.writeHead(404, { 'content-type': MIME['.txt']! });
    res.end('Not found');
    return;
  }
  // A symlink could still point outside the root.
  if (!isInside(realpathSync(resolve(root)), realpathSync(target))) {
    res.writeHead(403, { 'content-type': MIME['.txt']! });
    res.end('Forbidden');
    return;
  }
  const type = MIME[extname(target).toLowerCase()] ?? 'application/octet-stream';
  res.writeHead(200, { 'content-type': type });
  res.end(readFileSync(target));
}
