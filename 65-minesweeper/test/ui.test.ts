import { describe, it, expect } from 'vitest';
import { parseCommand } from '../src/ui/input';
import { renderBoard, cellGlyph } from '../src/ui/renderer';
import { Timer } from '../src/ui/timer';
import { boardFromLayout } from './helpers';

describe('input parser', () => {
  it('parses reveal and flag with aliases', () => {
    expect(parseCommand('r 3 4')).toEqual({ kind: 'reveal', r: 3, c: 4 });
    expect(parseCommand('reveal 0 0')).toEqual({ kind: 'reveal', r: 0, c: 0 });
    expect(parseCommand('f 1 2')).toEqual({ kind: 'flag', r: 1, c: 2 });
    expect(parseCommand('flag 5 6')).toEqual({ kind: 'flag', r: 5, c: 6 });
  });

  it('parses new game commands', () => {
    expect(parseCommand('new beginner')).toEqual({
      kind: 'new',
      difficulty: { rows: 9, cols: 9, mines: 10 },
    });
    expect(parseCommand('new custom 5 6 7')).toEqual({
      kind: 'new',
      difficulty: { rows: 5, cols: 6, mines: 7 },
    });
  });

  it('reports errors for malformed input', () => {
    expect(parseCommand('r 3').kind).toBe('error');
    expect(parseCommand('r a b').kind).toBe('error');
    expect(parseCommand('new wat').kind).toBe('error');
    expect(parseCommand('zzz').kind).toBe('error');
  });

  it('handles help, quit and empty input', () => {
    expect(parseCommand('help').kind).toBe('help');
    expect(parseCommand('quit').kind).toBe('quit');
    expect(parseCommand('   ').kind).toBe('noop');
  });
});

describe('renderer', () => {
  it('uses the documented glyphs', () => {
    const b = boardFromLayout(`
*..
...
..*
`);
    expect(cellGlyph(b.cells[0][0])).toBe('.'); // hidden
    b.cells[0][0].state = 'flagged';
    expect(cellGlyph(b.cells[0][0])).toBe('F');
    b.cells[1][1].state = 'revealed';
    expect(cellGlyph(b.cells[1][1])).toBe('2'); // two diagonal mines
    b.cells[0][1].state = 'revealed';
    expect(cellGlyph(b.cells[0][1])).toBe('1');
    // revealed mine on lost board
    expect(cellGlyph({ isMine: true, state: 'revealed', adjacent: 0 })).toBe('*');
  });

  it('renders a grid with headers without throwing', () => {
    const b = boardFromLayout(`
*..
...
`);
    const text = renderBoard(b);
    expect(text.split('\n').length).toBe(3); // header + 2 rows
  });
});

describe('timer', () => {
  it('starts at zero and only advances after start', () => {
    const t = new Timer();
    expect(t.elapsedSeconds(1000)).toBe(0);
    t.start(1000);
    expect(t.elapsedSeconds(4000)).toBe(3);
    t.start(2000); // start is idempotent
    expect(t.elapsedSeconds(4000)).toBe(3);
    t.reset();
    expect(t.elapsedSeconds(9999)).toBe(0);
  });
});
