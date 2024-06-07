import { createBunkers } from './bunkers.js';
import { resolveCollisions } from './collisions.js';
import type { GameConfig } from './config.js';
import { aliveCount, createFleet, fleetBottomY, updateFleet } from './fleet.js';
import {
  advanceBombs,
  advancePlayerShot,
  maybeDropBomb,
  spawnPlayerShot,
} from './projectiles.js';
import type { RNG } from './rng.js';
import { Mulberry32 } from './rng.js';
import { canFire, createShip, updateShip } from './ship.js';
import {
  createUfoController,
  maybeSpawnUfo,
  advanceUfo,
  type UfoController,
} from './ufo.js';
import type { GameEvent, GameState, InputState } from './types.js';

const DEFAULT_SEED = 0x1234567;

/**
 * The game simulation. It never reads the clock or Math.random, so the same
 * config, seed and inputs always play out the same way.
 */
export class Game {
  private readonly config: GameConfig;
  private readonly rng: RNG;
  private _state: GameState;
  private ufoCtrl: UfoController;
  private seed: number;

  constructor(config: GameConfig, rng?: RNG, seed: number = DEFAULT_SEED) {
    this.config = config;
    this.seed = seed >>> 0;
    this.rng = rng ?? new Mulberry32(this.seed);
    this.rng.reseed(this.seed);
    this.ufoCtrl = createUfoController();
    this._state = this.freshState(1);
  }

  get state(): Readonly<GameState> {
    return this._state;
  }

  /** Advance the simulation by dt seconds. Returns this tick's events. */
  step(input: InputState, dt: number): GameEvent[] {
    const events: GameEvent[] = [];
    const s = this._state;

    if (s.phase === 'gameOver') return events;
    if (s.phase === 'ready') s.phase = 'playing';
    if (s.phase !== 'playing') return events;

    updateShip(s.ship, input, dt, this.config);

    if (input.fire && canFire(s.ship, s.playerShot !== null)) {
      s.playerShot = spawnPlayerShot(s.ship, this.config);
      s.ship.fireCooldown = this.config.ship.fireCooldown;
      events.push({ type: 'shotFired' });
    }

    s.playerShot = advancePlayerShot(s.playerShot, dt, this.config);
    maybeDropBomb(s.fleet, s.bombs, dt, this.config, this.rng, events);
    s.bombs = advanceBombs(s.bombs, dt, this.config);

    updateFleet(s.fleet, dt, this.config, events);

    s.ufo = maybeSpawnUfo(s.ufo, this.ufoCtrl, dt, this.config, this.rng, events);
    s.ufo = advanceUfo(s.ufo, dt, this.config);

    resolveCollisions(s, this.config, events);

    if (s.ship.lives <= 0) {
      this.endGame(events);
      s.rngState = this.rng.state();
      return events;
    }

    if (aliveCount(s.fleet) === 0) {
      s.wave += 1;
      events.push({ type: 'waveCleared', wave: s.wave });
      s.fleet = createFleet(this.config, s.wave);
      s.playerShot = null;
      s.bombs = [];
      // Bunker damage carries over to the next wave, as in the arcade game.
      s.rngState = this.rng.state();
      return events;
    }

    // The fleet reaching the ship's row ends the game.
    if (fleetBottomY(s.fleet) >= s.ship.pos.y) {
      this.endGame(events);
    }

    s.rngState = this.rng.state();
    return events;
  }

  /** Restart at wave 1, optionally with a new seed. */
  reset(seed?: number): void {
    if (seed !== undefined) this.seed = seed >>> 0;
    this.rng.reseed(this.seed);
    this.ufoCtrl = createUfoController();
    this._state = this.freshState(1);
  }

  private endGame(events: GameEvent[]): void {
    this._state.phase = 'gameOver';
    events.push({ type: 'gameOver', score: this._state.score });
  }

  private freshState(wave: number): GameState {
    return {
      phase: 'playing',
      bounds: { ...this.config.bounds },
      ship: createShip(this.config),
      fleet: createFleet(this.config, wave),
      playerShot: null,
      bombs: [],
      bunkers: createBunkers(this.config),
      ufo: null,
      score: 0,
      wave,
      rngState: this.rng.state(),
    };
  }
}
