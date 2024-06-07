// Browser entry point: connects the canvas, keyboard and audio to the game.
import { AudioFx } from './adapters/audio.js';
import { draw } from './adapters/canvasRenderer.js';
import { KeyboardInput } from './adapters/keyboardInput.js';
import { RafClock } from './adapters/rafClock.js';
import { defaultConfig } from './core/config.js';
import { Game } from './core/game.js';
import { Mulberry32 } from './core/rng.js';

const FIXED_DT = 1 / 120;

function randomSeed(): number {
  return Math.floor(Math.random() * 0xffffffff) >>> 0;
}

function boot(): void {
  const canvas = document.getElementById('game') as HTMLCanvasElement | null;
  if (!canvas) throw new Error('canvas#game not found');
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('2d context unavailable');

  const config = defaultConfig();
  canvas.width = config.bounds.w;
  canvas.height = config.bounds.h;

  const seed = randomSeed();
  const game = new Game(config, new Mulberry32(seed), seed);
  const input = new KeyboardInput(window);
  const audio = new AudioFx(config.ufo.enabled); // no separate sound setting yet

  let started = false;

  const onStep = (dt: number): void => {
    if (!started) {
      if (input.consumeStart()) started = true;
      return;
    }
    if (game.state.phase === 'gameOver') {
      if (input.consumeStart()) game.reset(randomSeed());
      return;
    }
    const events = game.step(input.snapshot(), dt);
    audio.handle(events);
  };

  const onRender = (): void => {
    draw(ctx, game.state as never);
    if (!started) startOverlay(ctx, config.bounds.w, config.bounds.h);
  };

  const clock = new RafClock(onStep, onRender, FIXED_DT, 8);
  clock.start();
}

function startOverlay(ctx: CanvasRenderingContext2D, w: number, h: number): void {
  ctx.fillStyle = 'rgba(0,0,0,0.7)';
  ctx.fillRect(0, 0, w, h);
  ctx.fillStyle = '#e6edf3';
  ctx.textAlign = 'center';
  ctx.font = '40px monospace';
  ctx.fillText('SPACE INVADERS', w / 2, h / 2 - 40);
  ctx.font = '18px monospace';
  ctx.fillText('Move: ← → / A D    Fire: Space / ↑', w / 2, h / 2 + 10);
  ctx.fillText('Press Enter to start', w / 2, h / 2 + 40);
  ctx.textAlign = 'left';
}

if (typeof window !== 'undefined') {
  window.addEventListener('DOMContentLoaded', boot);
}
