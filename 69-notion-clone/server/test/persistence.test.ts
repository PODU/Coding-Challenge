import { describe, it, expect, afterEach } from 'vitest';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createStore } from '../src/store.js';
import { loadConfig } from '../src/config.js';
import { buildServices } from '../src/app.js';

describe('Persistence (R10 durability)', () => {
  const dirs: string[] = [];
  afterEach(() => {
    for (const d of dirs) rmSync(d, { recursive: true, force: true });
    dirs.length = 0;
  });

  it('writes via the service, reopens the file store, and reads data back', () => {
    const dir = mkdtempSync(join(tmpdir(), 'notion-test-'));
    dirs.push(dir);
    const file = join(dir, 'notion.json');
    const config = { ...loadConfig({}), dataPath: file };

    // First session: create user + page + block, then flush.
    let pageId: string;
    let userId: string;
    {
      const store = createStore(file);
      const services = buildServices(store, config);
      const { user } = services.auth.register('persist@x.com', 'secret123');
      userId = user.id;
      const page = services.pages.create(user.id, { title: 'Durable' });
      pageId = page.id;
      services.blocks.create(user.id, page.id, {
        type: 'heading_1',
        text: 'survives restart',
      });
      store.flush();
    }

    // Second session: brand-new store reading the same file.
    {
      const store = createStore(file);
      const services = buildServices(store, config);
      const detail = services.pages.get(userId, pageId);
      expect(detail.page.title).toBe('Durable');
      const texts = detail.blocks.map((b) => b.text);
      expect(texts).toContain('survives restart');
    }
  });
});
