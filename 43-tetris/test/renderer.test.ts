import { describe, it, expect, vi } from 'vitest';
import { CanvasRenderer } from '../src/shell/CanvasRenderer';
import { startGame } from './helpers';

/** Minimal mock of the bits of CanvasRenderingContext2D the renderer uses. */
function mockCtx() {
  return {
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 0,
    fillRect: vi.fn(),
    strokeRect: vi.fn(),
    clearRect: vi.fn(),
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    stroke: vi.fn(),
  };
}

describe('CanvasRenderer smoke test', () => {
  it('issues draw calls for a known state', () => {
    const renderer = new CanvasRenderer(30);
    const ctx = mockCtx();
    let s = startGame(1);
    s = { ...s, active: { id: 'O', rotation: 0, row: 0, col: 4 } };

    renderer.render(ctx as unknown as CanvasRenderingContext2D, s);

    // Background + grid lines + at least the active/ghost cells were drawn.
    expect(ctx.fillRect).toHaveBeenCalled();
    expect(ctx.stroke).toHaveBeenCalled();
    expect(ctx.fillRect.mock.calls.length).toBeGreaterThan(1);
  });

  it('renders the next-piece preview', () => {
    const renderer = new CanvasRenderer(30, 22);
    const ctx = mockCtx();
    renderer.renderNext(ctx as unknown as CanvasRenderingContext2D, 'T');
    expect(ctx.fillRect).toHaveBeenCalled();
  });
});
