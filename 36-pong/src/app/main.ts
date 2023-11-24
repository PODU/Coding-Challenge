import { defaultConfig } from '../core/config.js';
import { createGame, stepGame } from '../core/game.js';
import type { GameState } from '../core/types.js';
import { render } from '../render/canvasRenderer.js';
import { createKeyboard } from '../input/keyboard.js';

const FIXED_DT = 1 / 120; // seconds per physics step
const MAX_FRAME = 0.25; // cap the catch-up after e.g. a backgrounded tab

function main(): void {
  const canvas = document.getElementById('game') as HTMLCanvasElement | null;
  if (!canvas) throw new Error('canvas#game not found');
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('2D context unavailable');

  const cfg = defaultConfig;
  const rng = Math.random;
  const keyboard = createKeyboard();

  let state: GameState = createGame(cfg, rng);
  let last = performance.now();
  let acc = 0;

  function frame(now: number): void {
    let elapsed = (now - last) / 1000;
    last = now;
    if (elapsed > MAX_FRAME) elapsed = MAX_FRAME;
    acc += elapsed;

    // Fixed timestep, so the physics doesn't depend on the display's frame rate.
    while (acc >= FIXED_DT) {
      state = stepGame(state, keyboard.read(), cfg, rng, FIXED_DT);
      acc -= FIXED_DT;
    }

    render(ctx!, state, cfg);
    requestAnimationFrame(frame);
  }

  requestAnimationFrame(frame);
}

main();
