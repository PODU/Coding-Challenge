/**
 * Ties the session, keyboard and renderer together. Each animation frame it
 * ticks the session, tops up the prompt and repaints.
 */
import {
  type Clock,
  type TestConfig,
  TypingSession,
  WordProvider,
  COMMON_WORDS,
  makePRNG,
  wpm,
  rawWpm,
  accuracy,
  elapsedMinutes,
} from '../core/index.js';
import { Renderer } from './renderer.js';
import { KeyboardController } from './keyboard.js';

/** Add words whenever fewer than this many characters are left to type. */
const REFILL_AHEAD_CHARS = 120;

export class App {
  private readonly clock: Clock;
  private config: TestConfig;

  private provider: WordProvider;
  private session: TypingSession;
  private readonly renderer: Renderer;
  private readonly keyboard: KeyboardController;

  private rafId: number | null = null;
  private running = false;
  private resultsShown = false;

  constructor(root: HTMLElement, config: TestConfig, clock: Clock) {
    this.clock = clock;
    this.config = config;

    this.provider = this.makeProvider();
    this.session = new TypingSession(this.provider.nextWords(config.wordCount), config);
    this.renderer = new Renderer(root);
    this.keyboard = new KeyboardController(window, {
      onChar: (ch) => this.onChar(ch),
      onBackspace: () => this.onBackspace(),
      onRestart: () => this.restart(),
    });
  }

  start(): void {
    this.keyboard.attach();
    this.running = true;
    this.loop();
  }

  stop(): void {
    this.running = false;
    this.keyboard.detach();
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }

  /** Switch config and start a new test with it. */
  setConfig(config: TestConfig): void {
    this.config = config;
    this.provider = this.makeProvider();
    this.session = new TypingSession(this.provider.nextWords(config.wordCount), config);
    this.resultsShown = false;
  }

  /** Throw away progress and start a new prompt. */
  restart(): void {
    this.session.reset(this.provider.nextWords(this.config.wordCount));
    this.resultsShown = false;
  }

  private makeProvider(): WordProvider {
    const seed = this.config.seed ?? (Date.now() & 0xffffffff);
    return new WordProvider(COMMON_WORDS, makePRNG(seed));
  }

  private onChar(ch: string): void {
    this.session.typeChar(ch, this.clock.now());
    this.refillIfNeeded();
  }

  private onBackspace(): void {
    this.session.backspace(this.clock.now());
  }

  /** Add words before the user runs out of text to type. */
  private refillIfNeeded(): void {
    const snap = this.session.getState();
    if (snap.status === 'finished') return;
    if (snap.target.length - snap.cursor < REFILL_AHEAD_CHARS) {
      this.session.append(' ' + this.provider.nextWords(this.config.wordCount));
    }
  }

  private loop = (): void => {
    if (!this.running) return;
    const now = this.clock.now();
    this.session.tick(now);
    const snap = this.session.getState();

    if (snap.status === 'finished') {
      if (!this.resultsShown) {
        const end = snap.endMs ?? now;
        this.renderer.showResults(snap, {
          wpm: wpm(snap, end),
          raw: rawWpm(snap, end),
          acc: accuracy(snap),
        });
        this.resultsShown = true;
      }
    } else {
      const elapsedSec =
        snap.startMs === null ? 0 : elapsedMinutes(snap, now) * 60;
      const remainingSec = Math.max(0, snap.durationMs / 1000 - elapsedSec);
      this.renderer.render(snap, {
        wpm: wpm(snap, now),
        raw: rawWpm(snap, now),
        acc: accuracy(snap),
        remainingSec,
      });
    }

    this.rafId = requestAnimationFrame(this.loop);
  };
}
