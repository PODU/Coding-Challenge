// Shows the welcome, help, pause and game-over overlays and updates the HUD.

import type { GameState } from '../core/index';

export interface HudElements {
  score: HTMLElement;
  level: HTMLElement;
  lines: HTMLElement;
  overlay: HTMLElement;
  overlayTitle: HTMLElement;
  overlayBody: HTMLElement;
}

export function updateHud(state: GameState, els: HudElements): void {
  els.score.textContent = String(state.score);
  els.level.textContent = String(state.level);
  els.lines.textContent = String(state.lines);

  switch (state.phase) {
    case 'welcome':
      show(els, 'TETRIS', welcomeBody());
      break;
    case 'paused':
      show(els, 'Paused', 'Press <b>P</b> to resume.');
      break;
    case 'gameover':
      show(els, 'Game Over', `Final score: <b>${state.score}</b><br/>Press <b>Enter</b> to play again.`);
      break;
    case 'playing':
    default:
      hide(els);
      break;
  }
}

function welcomeBody(): string {
  return [
    'Clear lines. Don’t top out.',
    '',
    '<b>← →</b> move &nbsp; <b>↓</b> soft drop',
    '<b>↑ / X</b> rotate CW &nbsp; <b>Z</b> rotate CCW',
    '<b>Space</b> hard drop &nbsp; <b>P</b> pause',
    '',
    'Press <b>Enter</b> to start.',
  ].join('<br/>');
}

function show(els: HudElements, title: string, body: string): void {
  els.overlayTitle.innerHTML = title;
  els.overlayBody.innerHTML = body;
  els.overlay.style.display = 'flex';
}

function hide(els: HudElements): void {
  els.overlay.style.display = 'none';
}
