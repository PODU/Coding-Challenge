// Browser entry point: connects the core to the renderer, input and game loop.

import { createGame, reduce } from '../core/index';
import type { Action, GameState } from '../core/index';
import { CanvasRenderer } from './CanvasRenderer';
import { InputController } from './InputController';
import { GameLoop } from './GameLoop';
import { updateHud } from './screens';
import type { HudElements } from './screens';

const CELL = 30;

function el<T extends HTMLElement>(id: string): T {
  const node = document.getElementById(id);
  if (!node) throw new Error(`Missing element #${id}`);
  return node as T;
}

function randomSeed(): number {
  return (Math.floor(Math.random() * 0xffffffff) ^ Date.now()) | 0;
}

function main(): void {
  const boardCanvas = el<HTMLCanvasElement>('board');
  const nextCanvas = el<HTMLCanvasElement>('next');
  const boardCtx = boardCanvas.getContext('2d');
  const nextCtx = nextCanvas.getContext('2d');
  if (!boardCtx || !nextCtx) throw new Error('Canvas 2D context unavailable');

  const hud: HudElements = {
    score: el('score'),
    level: el('level'),
    lines: el('lines'),
    overlay: el('overlay'),
    overlayTitle: el('overlay-title'),
    overlayBody: el('overlay-body'),
  };

  const renderer = new CanvasRenderer(CELL);
  let state: GameState = createGame({ seed: randomSeed() });

  function dispatch(action: Action): void {
    state = reduce(state, action);
    render();
  }

  function render(): void {
    renderer.render(boardCtx!, state);
    renderer.renderNext(nextCtx!, state.nextQueue[0]);
    updateHud(state, hud);
  }

  const input = new InputController(dispatch);
  input.attach(window);

  // Phase-dependent keys are owned here because they need to read the phase.
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (state.phase === 'welcome') dispatch({ type: 'START' });
      else if (state.phase === 'gameover') dispatch({ type: 'RESTART' });
    } else if (e.key === 'p' || e.key === 'P') {
      e.preventDefault();
      if (state.phase === 'playing') dispatch({ type: 'PAUSE' });
      else if (state.phase === 'paused') dispatch({ type: 'RESUME' });
    }
  });

  const startBtn = document.getElementById('start-btn');
  startBtn?.addEventListener('click', () => {
    if (state.phase === 'welcome') dispatch({ type: 'START' });
    else if (state.phase === 'gameover') dispatch({ type: 'RESTART' });
  });

  const loop = new GameLoop((dtMs) => {
    if (state.phase === 'playing') {
      dispatch({ type: 'TICK', dtMs });
    }
  });
  loop.start();

  render();
}

main();
