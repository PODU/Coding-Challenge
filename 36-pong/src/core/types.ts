export type Vec2 = { x: number; y: number };

export type GamePhase = 'serving' | 'playing' | 'gameOver';

export type Side = 'left' | 'right';

export interface Config {
  table: { width: number; height: number };
  /** speed is units/second. */
  paddle: { width: number; height: number; speed: number };
  /** maxBounceAngle in radians, measured from the horizontal. */
  ball: { radius: number; baseSpeed: number; maxBounceAngle: number; maxSpeed: number };
  /** AI: speed units/sec, deadZone in units, error = vertical aim jitter in units. */
  ai: { speed: number; deadZone: number; error: number };
  winningScore: number;
  /** Seconds the ball waits at center before each launch. */
  serveDelay: number;
}

export interface InputState {
  /** Left paddle: -1 up, 0 none, 1 down. */
  playerDir: -1 | 0 | 1;
  /** Set for one step when the player asks to restart. */
  restart: boolean;
}

export interface GameState {
  ball: { pos: Vec2; vel: Vec2 };
  /** Paddle positions are their top edges. */
  leftPaddleY: number;
  rightPaddleY: number;
  scoreLeft: number;
  scoreRight: number;
  phase: GamePhase;
  winner: Side | null;
  /** Seconds until the serve, while phase is 'serving'. */
  serveDelay: number;
}

/** Returns values in [0, 1), like Math.random. */
export type Rng = () => number;
