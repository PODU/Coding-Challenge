import type { GameConfig } from './config.js';
import { clamp, type InputState, type Ship } from './types.js';

export function createShip(config: GameConfig): Ship {
  const { bounds, ship } = config;
  return {
    pos: {
      x: bounds.x + (bounds.w - ship.width) / 2,
      y: bounds.y + bounds.h - ship.marginBottom - ship.height,
    },
    w: ship.width,
    h: ship.height,
    lives: config.lives,
    fireCooldown: 0,
  };
}

/** Move the ship, keeping it on screen, and count down the fire cooldown. */
export function updateShip(ship: Ship, input: InputState, dt: number, config: GameConfig): void {
  let dir = 0;
  if (input.left) dir -= 1;
  if (input.right) dir += 1;

  ship.pos.x += dir * config.ship.speed * dt;
  ship.pos.x = clamp(
    ship.pos.x,
    config.bounds.x,
    config.bounds.x + config.bounds.w - ship.w,
  );

  if (ship.fireCooldown > 0) {
    ship.fireCooldown = Math.max(0, ship.fireCooldown - dt);
  }
}

/** Only one shot may be in flight, and the cooldown must have run out. */
export function canFire(ship: Ship, hasShotInFlight: boolean): boolean {
  return !hasShotInFlight && ship.fireCooldown <= 0;
}
