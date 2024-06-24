/** Terminal front end: reads commands, applies them to the board and redraws. */

import * as readline from 'node:readline';
import { createBoard, reveal, toggleFlag, isOver } from './core/board';
import type { Board } from './core/types';
import type { Difficulty } from './core/difficulty';
import { PRESETS, validateDifficulty } from './core/difficulty';
import { systemRng } from './core/rng';
import { renderBoard, renderStatus } from './ui/renderer';
import { parseCommand } from './ui/input';
import { Timer } from './ui/timer';

const HELP = `
Commands:
  r <row> <col>      reveal a cell      (alias: reveal)
  f <row> <col>      flag/unflag a cell (alias: flag)
  new <difficulty>   start a new game:
                       new beginner       (9x9, 10 mines)
                       new intermediate   (16x16, 40 mines)
                       new expert         (16x30, 99 mines)
                       new custom <rows> <cols> <mines>
  help               show this help      (alias: h, ?)
  quit               exit                (alias: q, exit)

Glyphs:  .  hidden    F  flag    *  mine    1-8  adjacent mines    (blank) zero
Rows and columns are 0-indexed; the first reveal is always safe.
`;

class Game {
  private board: Board;
  private readonly timer = new Timer();
  private readonly rng = systemRng();

  constructor(difficulty: Difficulty) {
    this.board = createBoard(difficulty);
  }

  newGame(difficulty: Difficulty): void {
    validateDifficulty(difficulty);
    this.board = createBoard(difficulty);
    this.timer.reset();
  }

  reveal(r: number, c: number): void {
    this.timer.start();
    this.board = reveal(this.board, r, c, this.rng);
  }

  flag(r: number, c: number): void {
    this.timer.start();
    this.board = toggleFlag(this.board, r, c);
  }

  render(): string {
    return `${renderBoard(this.board)}\n\n${renderStatus(
      this.board,
      this.timer.elapsedSeconds(),
    )}`;
  }

  get over(): boolean {
    return isOver(this.board);
  }
}

function main(): void {
  const game = new Game(PRESETS.beginner);
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  const prompt = (): void => {
    process.stdout.write('\n> ');
  };

  const draw = (): void => {
    console.log('\n' + game.render());
  };

  console.log('Minesweeper — Coding Challenge #65');
  console.log(HELP);
  draw();
  prompt();

  rl.on('line', (line) => {
    const cmd = parseCommand(line);
    switch (cmd.kind) {
      case 'noop':
        break;
      case 'error':
        console.log(cmd.message);
        break;
      case 'help':
        console.log(HELP);
        break;
      case 'quit':
        rl.close();
        return;
      case 'reveal':
        game.reveal(cmd.r, cmd.c);
        draw();
        if (game.over) console.log("\nType 'new <difficulty>' to play again, or 'quit' to exit.");
        break;
      case 'flag':
        game.flag(cmd.r, cmd.c);
        draw();
        break;
      case 'new':
        try {
          game.newGame(cmd.difficulty);
          draw();
        } catch (err) {
          console.log((err as Error).message);
        }
        break;
    }
    prompt();
  });

  rl.on('close', () => {
    console.log('\nThanks for playing!');
    process.exit(0);
  });
}

main();
