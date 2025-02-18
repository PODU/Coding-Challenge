/** File watching for live reload. The server does the actual push to browsers. */
import chokidar, { type FSWatcher } from 'chokidar';

export interface Watcher {
  close(): Promise<void>;
}

/** Call `onChange`, debounced, whenever anything under `paths` changes. */
export function watchPaths(
  paths: string[],
  onChange: () => void,
  debounceMs = 50,
): Watcher {
  const watcher: FSWatcher = chokidar.watch(paths, {
    ignoreInitial: true,
    awaitWriteFinish: { stabilityThreshold: 40, pollInterval: 10 },
  });

  let timer: NodeJS.Timeout | null = null;
  const trigger = (): void => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(onChange, debounceMs);
  };

  watcher.on('add', trigger);
  watcher.on('change', trigger);
  watcher.on('unlink', trigger);

  return {
    async close(): Promise<void> {
      if (timer) clearTimeout(timer);
      await watcher.close();
    },
  };
}

/** The only message the server sends over the live-reload socket. */
export const RELOAD_MESSAGE = JSON.stringify({ type: 'reload' });
