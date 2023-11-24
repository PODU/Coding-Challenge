import type { InputState } from '../core/types.js';

export interface KeyMap {
  up: string[];
  down: string[];
  restart: string[];
}

export const defaultKeyMap: KeyMap = {
  up: ['ArrowUp', 'w', 'W'],
  down: ['ArrowDown', 's', 'S'],
  restart: [' ', 'Enter'],
};

export interface Keyboard {
  /** Read the current input snapshot. `restart` latches until read once. */
  read(): InputState;
  dispose(): void;
}

/** Tracks held keys and turns them into an InputState. */
export function createKeyboard(keymap: KeyMap = defaultKeyMap): Keyboard {
  const held = new Set<string>();
  let restartLatched = false;

  const isUp = (k: string) => keymap.up.includes(k);
  const isDown = (k: string) => keymap.down.includes(k);
  const isRestart = (k: string) => keymap.restart.includes(k);

  const onKeyDown = (e: KeyboardEvent) => {
    if (isUp(e.key) || isDown(e.key) || isRestart(e.key)) e.preventDefault();
    if (isRestart(e.key) && !held.has(e.key)) restartLatched = true;
    held.add(e.key);
  };
  const onKeyUp = (e: KeyboardEvent) => {
    held.delete(e.key);
  };

  window.addEventListener('keydown', onKeyDown);
  window.addEventListener('keyup', onKeyUp);

  return {
    read(): InputState {
      // Holding up and down together cancels out.
      let dir: -1 | 0 | 1 = 0;
      const up = [...held].some(isUp);
      const down = [...held].some(isDown);
      if (up && !down) dir = -1;
      else if (down && !up) dir = 1;

      const restart = restartLatched;
      restartLatched = false;
      return { playerDir: dir, restart };
    },
    dispose() {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('keyup', onKeyUp);
      held.clear();
    },
  };
}
